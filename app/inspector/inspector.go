package inspector

import (
	"container/heap"
	"sync"
	"time"

	"github.com/zhangpanyi/basebot/logger"
	"github.com/zhangpanyi/basebot/telegram/methods"
	"github.com/zhangpanyi/basebot/telegram/updater"
	"github.com/zhangpanyi/luckymoney/app/config"
	"github.com/zhangpanyi/luckymoney/app/storage"
	"github.com/zhangpanyi/luckymoney/app/storage/models"
)

var once sync.Once
var inspector *Inspector

// 开始检查
func StartChecking(bot *methods.BotExt, pool *updater.Pool) {
	once.Do(func() {
		// 获取过期红包
		model := models.LuckyMoneyModel{}
		id, err := model.GetLatestExpired()
		if err != nil && err != storage.ErrNoBucket {
			logger.Panic(err)
		}

		// 遍历未过期列表
		h := make(heapExpire, 0)
		err = model.ForeachLuckyMoney(id+1, func(data *models.LuckyMoney) {
			heap.Push(&h, expire{ID: data.ID, Timestamp: data.Timestamp})
		})
		if err != nil && err != storage.ErrNoBucket {
			logger.Panic(err)
		}

		// 初始化红包检查器
		serverCfg := config.GetServe()
		inspector = &Inspector{
			h:      h,
			bot:    bot,
			pool:   pool,
			expire: serverCfg.Expire,
		}
		go inspector.loop()
	})
}

// 获取机器人
func GetBot() *methods.BotExt {
	return inspector.bot
}

// 添加红包
func AddToQueue(id uint64, timestamp int64) {
	inspector.lock.Lock()
	defer inspector.lock.Unlock()
	heap.Push(&inspector.h, expire{ID: id, Timestamp: timestamp})
}

// 检查员
type Inspector struct {
	h      heapExpire
	bot    *methods.BotExt
	pool   *updater.Pool
	lock   sync.RWMutex
	expire uint32
}

// 事件循环
func (t *Inspector) loop() {
	tickTimer := time.NewTimer(time.Second)
	for {
		select {
		case <-tickTimer.C:
			t.handleLuckyMoneyExpire()
			tickTimer.Reset(time.Second)
		}
	}
}

// 处理过期红包
func (t *Inspector) handleLuckyMoneyExpire() {
	var id uint64
	t.lock.RLock()
	now := time.Now().UTC().Unix()
	for t.h.Len() > 0 {
		data := t.h.Front()
		t.lock.RUnlock()

		// 判断是否过期
		if now-data.Timestamp < int64(t.expire) {
			return
		}

		// 获取过期信息
		t.lock.Lock()
		e := heap.Pop(&t.h).(expire)
		t.lock.Unlock()

		id = e.ID
		logger.Infof("Lucky money expired, %v", e.Timestamp)
		t.pool.Async(func() {
			t.asyncHandleLuckyMoneyExpire(e.ID)
		})
		t.lock.RLock()
	}
	t.lock.RUnlock()

	// 更新过期红包
	if id != 0 {
		models := models.LuckyMoneyModel{}
		if err := models.SetLatestExpired(id); err != nil {
			logger.Warnf("Failed to set last expired of lucky money, %v", err)
		}
	}
}

// 异步处理过期红包
func (t *Inspector) asyncHandleLuckyMoneyExpire(id uint64) {
	// 设置红包过期
	model := models.LuckyMoneyModel{}
	if model.IsExpired(id) {
		return
	}
	err := model.SetExpired(id)
	if err != nil {
		logger.Infof("Failed to set expired of lucky money, %v", err)
		return
	}

	// 获取红包信息
	luckyMoney, received, err := model.GetLuckyMoney(id)
	if err != nil {
		logger.Warnf("Failed to set expired of lucky money, not found lucky money, %d, %v", id, err)
		return
	}
	if received == luckyMoney.Number {
		return
	}

	// 计算红包余额
	balance := luckyMoney.Amount - luckyMoney.Received
	if !luckyMoney.Lucky {
		balance = luckyMoney.Amount*luckyMoney.Number - luckyMoney.Received
	}

	// 返还红包余额
	accountModel := models.AccountModel{}
	account, err := accountModel.UnlockAccount(luckyMoney.SenderID, luckyMoney.Asset, balance)
	if err != nil {
		logger.Errorf("Failed to return lucky money asset of expired, %v", err)
		return
	}
	logger.Errorf("Return lucky money asset of expired, UserID=%d, Asset=%s, Amount=%d",
		luckyMoney.SenderID, luckyMoney.Asset, balance)

	// 插入账户记录
	version := models.AccountVersionModel{}
	version.InsertVersion(luckyMoney.SenderID, &models.Version{
		Symbol:          luckyMoney.Asset,
		Locked:          -int32(balance),
		Amount:          account.Amount,
		Reason:          models.ReasonGiveBack,
		RefLuckyMoneyID: &luckyMoney.ID,
	})
}

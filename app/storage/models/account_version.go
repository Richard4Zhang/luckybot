package models

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/zhangpanyi/luckymoney/app/storage"
)

// 触发原因
type Reason int

const (
	_                     Reason = iota
	ReasonGive                   // 发红包
	ReasonReceive                // 领取红包
	ReasonWithdraw               // 提现
	ReasonWithdrawFailure        // 提现失败
	ReasonGiveBack               // 退还红包
	ReasonDeposit                // 充值
	ReasonWithdrawSuccess        // 提现成功
)

// 版本信息
type Version struct {
	ID              int64   `json:"id"`                           // 版本ID
	Symbol          string  `json:"symbol"`                       // 代币符号
	Balance         int32   `json:"balance"`                      // 余额变化
	Locked          int32   `json:"locked"`                       // 锁定变化
	Fee             uint32  `json:"fee"`                          // 手续费
	Amount          uint32  `json:"amount"`                       // 剩余金额
	Timestamp       int64   `json:"Timestamp"`                    // 时间戳
	Reason          Reason  `json:"reason"`                       // 触发原因
	RefLuckyMoneyID *uint64 `json:"ref_lucky_money_id,omitempty"` // 关联红包ID
	RefBlockHeight  *uint64 `json:"ref_block_height,omitempty"`   // 关联区块高度
	RefTxID         *string `json:"ref_tx_id,omitempty"`          // 关联交易ID
	RefUserID       *int64  `json:"ref_user_id,omitempty"`        // 关联用户ID
	RefUserName     *string `json:"ref_user_name,omitempty"`      // 关联用户名
	RefOrderID      *string `json:"ref_order_id,omitempty"`       // 关联订单ID
	RefAddress      *string `json:"ref_address,omitempty"`        // 关联地址
	RefMemo         *string `json:"ref_memo,omitempty"`           // 关联备注信息
}

// ********************** 结构图 **********************
// {
//	"account_versions": {
// 		<user_id>: {
// 			<seq>: Version	// 版本信息
// 		}
//	}
// ***************************************************

// 账户版本模型
type AccountVersionModel struct {
}

// 插入版本
func (model *AccountVersionModel) InsertVersion(userID int64, version *Version) error {
	version.Timestamp = time.Now().UTC().Unix()
	jsb, err := json.Marshal(version)
	if err != nil {
		return err
	}
	key := strconv.FormatInt(userID, 10)
	err = storage.DB.Update(func(tx *bolt.Tx) error {
		bucket, err := storage.EnsureBucketExists(tx, "account_versions", key)
		if err != nil {
			return err
		}

		seq, err := bucket.NextSequence()
		if err != nil {
			return err
		}
		return bucket.Put([]byte(strconv.FormatUint(seq, 10)), jsb)
	})
	return nil
}

// 获取版本
func (model *AccountVersionModel) GetVersions(userID int64, offset, limit uint, reverse bool) ([]*Version, int, error) {
	sum := 0
	jsonarray := make([][]byte, 0)
	key := strconv.FormatInt(userID, 10)
	err := storage.DB.Update(func(tx *bolt.Tx) error {
		bucket, err := storage.GetBucketIfExists(tx, "account_versions", key)
		if err != nil {
			if err != storage.ErrNoBucket {
				return err
			}
			return nil
		}

		var idx uint
		filter := func(idx uint, k, v []byte) bool {
			if v != nil {
				if idx >= offset {
					jsonarray = append(jsonarray, v)
					if len(jsonarray) >= int(limit) {
						return false
					}
				}
				idx++
			}
			return true
		}

		cursor := bucket.Cursor()
		if reverse {
			for k, v := cursor.Last(); k != nil; k, v = cursor.Prev() {
				if !filter(idx, k, v) {
					break
				}
				idx++
			}
		} else {
			for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
				if !filter(idx, k, v) {
					break
				}
				idx++
			}
		}
		sum = bucket.Stats().KeyN
		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	versions := make([]*Version, 0)
	for i := 0; i < len(jsonarray); i++ {
		jsb := jsonarray[i]
		var version Version
		if err = json.Unmarshal(jsb, &version); err != nil {
			return nil, 0, err
		}
		versions = append(versions, &version)
	}
	return versions, sum, nil
}

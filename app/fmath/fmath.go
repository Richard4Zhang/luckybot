package fmath

import (
	"math/big"
)

// 相加
func Add(x *big.Float, y *big.Float) *big.Float {
	return big.NewFloat(0).Add(x, y)
}

// 相减
func Sub(x *big.Float, y *big.Float) *big.Float {
	return big.NewFloat(0).Sub(x, y)
}

// 相乘
func Mul(x *big.Float, y *big.Float) *big.Float {
	return big.NewFloat(0).Mul(x, y)
}

// 取绝对值
func Abs(x *big.Float) *big.Float {
	return big.NewFloat(0).Abs(x)
}

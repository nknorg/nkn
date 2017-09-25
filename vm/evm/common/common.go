package common

import (
	"math/big"
)

var (
	Big0      = big.NewInt(0)
	Big1      = big.NewInt(1)
	Big32     = big.NewInt(32)
	tt255     = BigPow(2, 255)
	tt256     = BigPow(2, 256)
	tt256m1   = new(big.Int).Sub(tt256, big.NewInt(1))
	wordBits  = 32 << (uint64(^big.Word(0)) >> 63)
	wordBytes = wordBits / 8
)

func BigPow(a, b int64) *big.Int {
	r := big.NewInt(a)
	return r.Exp(r, big.NewInt(b), nil)
}

func GetData(data []byte, start, size *big.Int) []byte {
	dlen := big.NewInt(int64(len(data)))
	s := BigMin(start, dlen)
	e := BigMin(new(big.Int).Add(s, size), dlen)
	return RightPadBytes(data[s.Uint64():e.Uint64()], int(size.Uint64()))
}

func RightPadBytes(slice []byte, l int) []byte {
	if l <= len(slice) {
		return slice
	}

	padded := make([]byte, l)
	copy(padded, slice)

	return padded
}

func LeftPadBytes(slice []byte, l int) []byte {
	if l <= len(slice) {
		return slice
	}

	padded := make([]byte, l)
	copy(padded[l-len(slice):], slice)

	return padded
}

func BigMax(x, y *big.Int) *big.Int {
	if x.Cmp(y) < 0 {
		return y
	}
	return x
}

func BigMin(x, y *big.Int) *big.Int {
	if x.Cmp(y) > 0 {
		return y
	}
	return x
}

func PaddedBigBytes(bigint *big.Int, n int) []byte {
	if bigint.BitLen()/8 >= n {
		return bigint.Bytes()
	}
	ret := make([]byte, n)
	ReadBits(bigint, ret)
	return ret
}

func ReadBits(bigint *big.Int, buf []byte) {
	i := len(buf)
	for _, d := range bigint.Bits() {
		for j := 0; j < wordBytes && i > 0; j++ {
			i--
			buf[i] = byte(d)
			d >>= 8
		}
	}
}

func CalcMemSize(off, l *big.Int) *big.Int {
	if l.Sign() == 0 {
		return Big0
	}

	return new(big.Int).Add(off, l)
}

func Exp(base, exponent *big.Int) *big.Int {
	result := big.NewInt(1)

	for _, word := range exponent.Bits() {
		for i := 0; i < wordBits; i++ {
			if word&1 == 1 {
				U256(result.Mul(result, base))
			}
			U256(base.Mul(base, base))
			word >>= 1
		}
	}
	return result
}

func U256(x *big.Int) *big.Int {
	return x.And(x, tt256m1)
}

func S256(x *big.Int) *big.Int {
	if x.Cmp(tt255) < 0 {
		return x
	} else {
		return new(big.Int).Sub(x, tt256)
	}
}




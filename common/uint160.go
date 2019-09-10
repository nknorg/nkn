package common

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	base58 "github.com/itchyny/base58-go"
)

// FOOLPROOFPREFIX used for fool-proof prefix
// base58.BitcoinEncoding[21] = 'N', base58.BitcoinEncoding[18] = 'K'
// 33 = len(base58.Encode( (2**192).Bytes() )),  192 = 8bit * (UINT160SIZE + SHA256CHKSUM)
// ((21 * 58**35) + (18 * 58**34) + (21 * 58**33)) >> 192 = 0x02b824
const FOOLPROOFPREFIX = 0x02b824 + 1 // +1 for avoid affected by lower 192bits shift-add

// PREFIXLEN = len( 0x02b825.Bytes() )
const PREFIXLEN = 3
const UINT160SIZE = 20
const SHA256CHKSUM = 4
const HEXADDRLEN = PREFIXLEN + UINT160SIZE + SHA256CHKSUM

type Uint160 [UINT160SIZE]uint8

var EmptyUint160 Uint160

func (u *Uint160) CompareTo(o Uint160) int {
	x := u.ToArray()
	y := o.ToArray()

	for i := 0; i < len(x); i++ {
		if x[i] > y[i] {
			return 1
		}
		if x[i] < y[i] {
			return -1
		}
	}

	return 0
}

func (u *Uint160) ToArray() []byte {
	var x = make([]byte, UINT160SIZE)
	for i := 0; i < UINT160SIZE; i++ {
		x[i] = byte(u[i])
	}

	return x
}

func (u *Uint160) Serialize(w io.Writer) (int, error) {
	b_buf := bytes.NewBuffer([]byte{})
	binary.Write(b_buf, binary.LittleEndian, u)

	len, err := w.Write(b_buf.Bytes())

	if err != nil {
		return 0, err
	}

	return len, nil
}

func (f *Uint160) Deserialize(r io.Reader) error {
	p := make([]byte, UINT160SIZE)
	n, err := r.Read(p)

	if n <= 0 || err != nil {
		return err
	}

	b_buf := bytes.NewBuffer(p)
	binary.Read(b_buf, binary.LittleEndian, f)

	return nil
}

func IsValidHexAddr(s []byte) bool {
	if len(s) == HEXADDRLEN && new(big.Int).SetBytes(s[:PREFIXLEN]).Uint64() == FOOLPROOFPREFIX {
		sha := sha256.Sum256(s[:PREFIXLEN+UINT160SIZE])
		chkSum := sha256.Sum256(sha[:])
		return bytes.Compare(s[PREFIXLEN+UINT160SIZE:], chkSum[:SHA256CHKSUM]) == 0
	}
	return false
}

func (f Uint160) MarshalJSON() ([]byte, error) {
	str, err := f.ToAddress()
	return []byte("\"" + str + "\""), err
}

func (f *Uint160) ToAddress() (string, error) {
	data := append(big.NewInt(FOOLPROOFPREFIX).Bytes(), f.ToArray()...)
	temp := sha256.Sum256(data)
	temps := sha256.Sum256(temp[:])
	data = append(data, temps[0:SHA256CHKSUM]...)

	bi := new(big.Int).SetBytes(data).String()
	encoding := base58.BitcoinEncoding
	encoded, err := encoding.Encode([]byte(bi))
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func Uint160ParseFromBytes(f []byte) (Uint160, error) {
	if len(f) != UINT160SIZE {
		return EmptyUint160, errors.New("[Common]: Uint160ParseFromBytes err, len != 20")
	}

	var hash [UINT160SIZE]uint8
	for i := 0; i < UINT160SIZE; i++ {
		hash[i] = f[i]
	}
	return Uint160(hash), nil
}

func ToScriptHash(address string) (Uint160, error) {
	encoding := base58.BitcoinEncoding

	decoded, err := encoding.Decode([]byte(address))
	if err != nil {
		return EmptyUint160, err
	}

	bint, ok := new(big.Int).SetString(string(decoded), 10)
	if !ok {
		return EmptyUint160, fmt.Errorf("Base58.Decode[%s] %s NOT a decimal number", address, decoded)
	}

	hex := bint.Bytes()
	if !IsValidHexAddr(hex) {
		return EmptyUint160, fmt.Errorf("address[%s] decode %x not a valid address", address, hex)
	}

	return Uint160ParseFromBytes(hex[PREFIXLEN : PREFIXLEN+UINT160SIZE])
}

func (u *Uint160) SetBytes(b []byte) *Uint160 {
	if len(b) > len(u) {
		b = b[len(b)-UINT160SIZE:]
	}
	copy(u[UINT160SIZE-len(b):], b)
	return u
}

func BytesToUint160(b []byte) Uint160 {
	u := new(Uint160)
	u.SetBytes(b)
	return *u
}

func BigToUint160(b *big.Int) Uint160 {
	return BytesToUint160(b.Bytes())
}

func (u *Uint160) Big() *big.Int {
	return new(big.Int).SetBytes(u.ToArray()[:])
}

func (u *Uint160) ToHexString() string {
	return fmt.Sprintf("%x", u[:])
}

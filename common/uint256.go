package common

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const UINT256SIZE = 32

type Uint256 [UINT256SIZE]uint8

var EmptyUint256 Uint256
var MaxUint256 = Uint256{
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
}

func (u *Uint256) CompareTo(o Uint256) int {
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

func (u *Uint256) ToArray() []byte {
	return u[:]
}

func (u *Uint256) Serialize(w io.Writer) (int, error) {
	b_buf := bytes.NewBuffer([]byte{})
	binary.Write(b_buf, binary.LittleEndian, u)

	len, err := w.Write(b_buf.Bytes())

	if err != nil {
		return 0, err
	}

	return len, nil
}

func (u *Uint256) Deserialize(r io.Reader) error {
	p := make([]byte, UINT256SIZE)
	n, err := r.Read(p)

	if n <= 0 || err != nil {
		return err
	}

	b_buf := bytes.NewBuffer(p)
	binary.Read(b_buf, binary.LittleEndian, u)

	return nil
}

func (u *Uint256) ToString() string {
	return string(u.ToArray())
}

func (u *Uint256) ToHexString() string {
	return fmt.Sprintf("%x", u.ToArray())
}

func Uint256ParseFromBytes(f []byte) (hash Uint256, err error) {
	copy(hash[:], f)
	if len(f) != UINT256SIZE {
		err = fmt.Errorf("uint256 bytes wrong size %d, expecting %d", len(f), UINT256SIZE)
	}

	return hash, err
}

func U256Equal(a, b Uint256) bool {
	return bytes.Equal(a[:], b[:])
}

package common

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"strings"
)

const (
	// supported max asset precision is 8
	MaximumPrecision = 8
	StorageFactor    = 100000000
)

//the 64 bit fixed-point number, precise 10^-8
type Fixed64 int64

func (f *Fixed64) Serialize(w io.Writer) error {
	err := binary.Write(w, binary.LittleEndian, int64(*f))
	if err != nil {
		return err
	}

	return nil
}

func (f *Fixed64) Deserialize(r io.Reader) error {
	p := make([]byte, 8)
	n, err := r.Read(p)
	if n <= 0 || err != nil {
		return err
	}
	b_buf := bytes.NewBuffer(p)
	var x int64
	err = binary.Read(b_buf, binary.LittleEndian, &x)
	if err != nil {
		return err
	}
	*f = Fixed64(x)
	return nil
}

func (f Fixed64) GetData() int64 {
	return int64(f)
}

func (f Fixed64) String() string {
	var buffer bytes.Buffer
	value := uint64(f)
	if f < 0 {
		buffer.WriteRune('-')
		value = uint64(-f)
	}
	buffer.WriteString(strconv.FormatUint(value/StorageFactor, 10))
	value %= StorageFactor
	if value > 0 {
		buffer.WriteRune('.')
		s := strconv.FormatUint(value, 10)
		for i := len(s); i < 8; i++ {
			buffer.WriteRune('0')
		}
		buffer.WriteString(s)
	}
	return buffer.String()
}

func StringToFixed64(s string) (Fixed64, error) {
	var buffer bytes.Buffer

	di := strings.Index(s, ".")
	if di == -1 {
		buffer.WriteString(s)
		for i := 0; i < MaximumPrecision; i++ {
			buffer.WriteByte('0')
		}
	} else {
		precision := len(s) - di - 1
		if precision > MaximumPrecision {
			return Fixed64(0), errors.New("unsupported precision")
		}
		buffer.WriteString(s[:di])
		buffer.WriteString(s[di+1:])
		n := MaximumPrecision - precision
		for i := 0; i < n; i++ {
			buffer.WriteByte('0')
		}
	}
	r, err := strconv.ParseInt(buffer.String(), 10, 64)
	if err != nil {
		return Fixed64(0), err
	}

	return Fixed64(r), nil
}

package common

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"

	"golang.org/x/crypto/ripemd160"
)

const MaxUint32 = ^uint32(0)

func ToCodeHash(code []byte) (Uint160, error) {
	temp := sha256.Sum256(code)
	md := ripemd160.New()
	io.WriteString(md, string(temp[:]))
	f := md.Sum(nil)

	hash, err := Uint160ParseFromBytes(f)
	if err != nil {
		return Uint160{}, errors.New("ToCodehash parse uint160 error")
	}
	return hash, nil
}

func IntToBytes(n int) []byte {
	tmp := int32(n)
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.LittleEndian, tmp)
	return bytesBuffer.Bytes()
}

func BytesToInt16(b []byte) int16 {
	bytesBuffer := bytes.NewBuffer(b)
	var tmp int16
	binary.Read(bytesBuffer, binary.BigEndian, &tmp)
	return int16(tmp)
}

func BytesToInt(b []byte) []int {
	i := make([]int, len(b))
	for k, v := range b {
		i[k] = int(v)
	}
	return i
}

func BytesToHexString(data []byte) string {
	return hex.EncodeToString(data)
}

func HexStringToBytes(value string) ([]byte, error) {
	return hex.DecodeString(value)
}

func ClearBytes(arr []byte, len int) {
	for i := 0; i < len; i++ {
		arr[i] = 0
	}
}

func CompareHeight(blockHeight uint32, heights []uint32) bool {
	for _, height := range heights {
		if blockHeight < height {
			return false
		}
	}
	return true
}

func GetUint16Array(source []byte) ([]uint16, error) {
	if source == nil {
		return nil, errors.New("[Common] , GetUint16Array err, source = nil")
	}

	if len(source)%2 != 0 {
		return nil, errors.New("[Common] , GetUint16Array err, length of source is odd.")
	}

	dst := make([]uint16, len(source)/2)
	for i := 0; i < len(source)/2; i++ {
		dst[i] = uint16(source[i*2]) + uint16(source[i*2+1])*256
	}

	return dst, nil
}

func ToByteArray(source []uint16) []byte {
	dst := make([]byte, len(source)*2)
	for i := 0; i < len(source); i++ {
		dst[i*2] = byte(source[i] % 256)
		dst[i*2+1] = byte(source[i] / 256)
	}

	return dst
}

func SliceRemove(slice []uint32, h uint32) []uint32 {
	for i, v := range slice {
		if v == h {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func FileExisted(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

func AbsUint(n1 uint, n2 uint) uint {
	if n1 > n2 {
		return n1 - n2
	}
	return n2 - n1
}

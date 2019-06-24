package helpers

import (
	"bytes"
	"io/ioutil"
)

func LoadFile(path string) ([]byte, error) {
	file, err := ioutil.ReadFile(path)
	if err == nil {
		// Remove the UTF-8 Byte Order Mark
		file = bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))
	}
	return file, err
}

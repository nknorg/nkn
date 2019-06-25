package helpers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func BuildPwd(pwd string) []byte {
	key := []byte(pwd)
	h := hmac.New(sha256.New, key)
	h.Write(key)

	return h.Sum(nil)
}

func AesEncrypt(plaintext string, pwd string) (string, error) {
	key := BuildPwd(pwd)
	var iv = key[:aes.BlockSize]

	encrypted := make([]byte, len(plaintext))
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	encrypter := cipher.NewCFBEncrypter(block, iv)
	encrypter.XORKeyStream(encrypted, []byte(plaintext))
	return hex.EncodeToString(encrypted), nil
}

func AesDecrypt(encrypted string, pwd string) (string, error) {
	key := BuildPwd(pwd)
	var err error
	defer func() {
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()
	src, err := hex.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	var iv = key[:aes.BlockSize]
	decrypted := make([]byte, len(src))
	var block cipher.Block
	block, err = aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	decrypter := cipher.NewCFBDecrypter(block, iv)
	decrypter.XORKeyStream(decrypted, src)
	return string(decrypted), nil
}

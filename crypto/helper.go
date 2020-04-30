package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"errors"

	"github.com/nknorg/nkn/common"
)

func PasswordHash(password []byte) []byte {
	passwordhash := sha256.Sum256(password)
	passwordhash2 := sha256.Sum256(passwordhash[:])
	passwordhash3 := sha256.Sum256(passwordhash2[:])
	common.ClearBytes(passwordhash[:])
	common.ClearBytes(passwordhash2[:])
	return passwordhash3[:]
}

func AesEncrypt(plaintext []byte, key []byte, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("invalid decrypt key")
	}

	ciphertext := make([]byte, len(plaintext))
	blockMode := cipher.NewCBCEncrypter(block, iv)
	blockMode.CryptBlocks(ciphertext, plaintext)

	return ciphertext, nil
}

func AesDecrypt(ciphertext []byte, key []byte, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("invalid decrypt key")
	}

	blockSize := block.BlockSize()

	if len(ciphertext) < blockSize {
		return nil, errors.New("ciphertext too short")
	}

	if len(ciphertext)%blockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	plaintext := make([]byte, len(ciphertext))
	blockMode := cipher.NewCBCDecrypter(block, iv)
	blockMode.CryptBlocks(plaintext, ciphertext)

	return plaintext, nil
}

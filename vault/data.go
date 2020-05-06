package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/util"
	"golang.org/x/crypto/scrypt"
)

const (
	IVLength                   = 16
	MasterKeyLength            = 32
	ScryptSaltLength           = 8
	ScryptN                    = 1 << 15
	ScryptR                    = 8
	ScryptP                    = 1
	WalletVersion              = 2
	MinCompatibleWalletVersion = 1
	MaxCompatibleWalletVersion = 2
)

type ScryptData struct {
	Salt string
	N    int
	R    int
	P    int
}

type WalletData struct {
	Version       int
	IV            string
	MasterKey     string
	SeedEncrypted string
	Address       string
	Scrypt        *ScryptData
}

func NewWalletData(account *Account, password, masterKey, iv, scryptSalt []byte, scryptN, scryptR, scryptP int) (*WalletData, error) {
	if len(masterKey) == 0 {
		masterKey = util.RandomBytes(MasterKeyLength)
	}

	if len(iv) == 0 {
		iv = util.RandomBytes(IVLength)
	}

	if len(scryptSalt) == 0 {
		scryptSalt = util.RandomBytes(ScryptSaltLength)
	}

	if scryptN == 0 {
		scryptN = ScryptN
	}

	if scryptR == 0 {
		scryptR = ScryptR
	}

	if scryptP == 0 {
		scryptP = ScryptP
	}

	scryptData := &ScryptData{
		Salt: hex.EncodeToString(scryptSalt),
		N:    scryptN,
		R:    scryptR,
		P:    scryptP,
	}

	var passwordKey []byte
	var err error
	switch WalletVersion {
	case 1:
		passwordKey = PasswordToAesKeyHash(password)
	case 2:
		passwordKey, err = PasswordToAesKeyScrypt(password, scryptData)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported wallet version %d", WalletVersion)
	}

	encryptedMasterKey, err := crypto.AesEncrypt(masterKey, passwordKey, iv)
	if err != nil {
		return nil, err
	}

	seed := crypto.GetSeedFromPrivateKey(account.PrivateKey)
	encryptedSeed, err := crypto.AesEncrypt(seed, masterKey, iv)
	if err != nil {
		return nil, err
	}

	address, err := account.ProgramHash.ToAddress()
	if err != nil {
		return nil, err
	}

	walletData := &WalletData{
		Version:       WalletVersion,
		IV:            hex.EncodeToString(iv),
		MasterKey:     hex.EncodeToString(encryptedMasterKey),
		SeedEncrypted: hex.EncodeToString(encryptedSeed),
		Address:       address,
		Scrypt:        scryptData,
	}

	return walletData, nil
}

func (walletData *WalletData) DecryptMasterKey(password []byte) ([]byte, error) {
	var passwordKey []byte
	var err error
	switch walletData.Version {
	case 1:
		passwordKey = PasswordToAesKeyHash(password)
	case 2:
		passwordKey, err = PasswordToAesKeyScrypt(password, walletData.Scrypt)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported wallet version %d", walletData.Version)
	}

	iv, err := hex.DecodeString(walletData.IV)
	if err != nil {
		return nil, err
	}

	encryptedMasterKey, err := hex.DecodeString(walletData.MasterKey)
	if err != nil {
		return nil, err
	}

	return crypto.AesDecrypt(encryptedMasterKey, passwordKey, iv)
}

func (walletData *WalletData) DecryptAccount(password []byte) (*Account, error) {
	masterKey, err := walletData.DecryptMasterKey(password)
	if err != nil {
		return nil, err
	}

	encryptedSeed, err := hex.DecodeString(walletData.SeedEncrypted)
	if err != nil {
		return nil, err
	}

	iv, err := hex.DecodeString(walletData.IV)
	if err != nil {
		return nil, err
	}

	seed, err := crypto.AesDecrypt(encryptedSeed, masterKey, iv)
	if err != nil {
		return nil, err
	}

	return NewAccountWithSeed(seed)
}

func (walletData *WalletData) VerifyPassword(password []byte) error {
	account, err := walletData.DecryptAccount(password)
	if err != nil {
		return err
	}

	address, err := account.ProgramHash.ToAddress()
	if err != nil {
		return err
	}

	if address != walletData.Address {
		return errors.New("wrong password")
	}

	return nil
}

func PasswordToAesKeyHash(password []byte) []byte {
	passwordhash := sha256.Sum256(password)
	passwordhash2 := sha256.Sum256(passwordhash[:])
	common.ClearBytes(passwordhash[:])
	return passwordhash2[:]
}

func PasswordToAesKeyScrypt(password []byte, scryptData *ScryptData) ([]byte, error) {
	salt, err := hex.DecodeString(scryptData.Salt)
	if err != nil {
		return nil, err
	}
	return scrypt.Key(password, salt, scryptData.N, scryptData.R, scryptData.P, 32)
}

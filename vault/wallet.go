package vault

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	serviceConfig "github.com/nknorg/nkn/dashboard/config"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/program"
	"github.com/nknorg/nkn/signature"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/password"
)

const (
	WalletIVLength             = 16
	WalletMasterKeyLength      = 32
	WalletVersion              = 1
	MinCompatibleWalletVersion = 1
	MaxCompatibleWalletVersion = 1
)

type Wallet interface {
	Sign(txn *transaction.Transaction) error
	GetAccount(pubKey *crypto.PubKey) (*Account, error)
	GetDefaultAccount() (*Account, error)
}

type WalletImpl struct {
	path      string
	iv        []byte
	masterKey []byte
	account   *Account
	contract  *program.ProgramContext
	*WalletStore
}

func NewWallet(path string, password []byte, needAccount bool) (*WalletImpl, error) {
	var err error
	// store init
	store, err := NewStore(path)
	if err != nil {
		return nil, err
	}
	// generate password hash
	passwordKey := crypto.ToAesKey(password)
	pwdhash := sha256.Sum256(passwordKey)
	// generate IV
	iv := make([]byte, WalletIVLength)
	_, err = rand.Read(iv)
	if err != nil {
		return nil, err
	}
	// generate master key
	masterKey := make([]byte, WalletMasterKeyLength)
	_, err = rand.Read(masterKey)
	if err != nil {
		return nil, err
	}
	encryptedMasterKey, err := crypto.AesEncrypt(masterKey[:], passwordKey, iv)
	if err != nil {
		return nil, err
	}
	// persist to store
	err = store.SaveBasicData(WalletVersion, iv, encryptedMasterKey, pwdhash[:])
	if err != nil {
		return nil, err
	}

	w := &WalletImpl{
		path:        path,
		iv:          iv,
		masterKey:   masterKey,
		WalletStore: store,
	}
	// generate default account
	if needAccount {
		err = w.CreateAccount(nil)
		if err != nil {
			return nil, err
		}
	}

	return w, nil
}

func OpenWallet(path string, password []byte) (*WalletImpl, error) {
	var err error
	store, err := LoadStore(path)
	if err != nil {
		return nil, err
	}

	if store.Data.Version < MinCompatibleWalletVersion || store.Data.Version > MaxCompatibleWalletVersion {
		return nil, fmt.Errorf("invalid wallet version %v, should be between %v and %v", store.Data.Version, MinCompatibleWalletVersion, MaxCompatibleWalletVersion)
	}

	passwordKey := crypto.ToAesKey(password)
	passwordKeyHash, err := HexStringToBytes(store.Data.PasswordHash)
	if err != nil {
		return nil, err
	}
	if ok := verifyPasswordKey(passwordKey, passwordKeyHash); !ok {
		return nil, errors.New("password wrong")
	}
	iv, err := HexStringToBytes(store.Data.IV)
	if err != nil {
		return nil, err
	}
	encryptedMasterKey, err := HexStringToBytes(store.Data.MasterKey)
	if err != nil {
		return nil, err
	}
	masterKey, err := crypto.AesDecrypt(encryptedMasterKey, passwordKey, iv)
	if err != nil {
		return nil, err
	}

	encryptedSeed, err := HexStringToBytes(store.Data.AccountData.SeedEncrypted)
	if err != nil {
		return nil, err
	}
	seed, err := crypto.AesDecrypt(encryptedSeed, masterKey, iv)
	if err != nil {
		return nil, err
	}

	privateKey := crypto.GetPrivateKeyFromSeed(seed)
	if err = crypto.CheckPrivateKey(privateKey); err != nil {
		return nil, err
	}

	account, err := NewAccountWithPrivatekey(privateKey)
	if err != nil {
		return nil, err
	}

	ct, err := program.CreateSignatureProgramContext(account.PubKey())
	if err != nil {
		return nil, err
	}

	return &WalletImpl{
		path:        path,
		iv:          iv,
		masterKey:   masterKey,
		account:     account,
		contract:    ct,
		WalletStore: store,
	}, nil
}

func RecoverWallet(path string, password []byte, seedHex string) (*WalletImpl, error) {
	wallet, err := NewWallet(path, password, false)
	if err != nil {
		return nil, errors.New("create new wallet error")
	}
	seed, err := HexStringToBytes(seedHex)
	if err != nil {
		return nil, err
	}
	err = wallet.CreateAccount(seed)
	if err != nil {
		return nil, err
	}

	return wallet, nil
}

func (w *WalletImpl) CreateAccount(seed []byte) error {
	var account *Account
	var err error
	if seed == nil {
		account, err = NewAccount()
		if err != nil {
			return err
		}
		seed = crypto.GetSeedFromPrivateKey(account.PrivateKey)
	} else {
		if err = crypto.CheckSeed(seed); err != nil {
			return err
		}
		privateKey := crypto.GetPrivateKeyFromSeed(seed)
		if err = crypto.CheckPrivateKey(privateKey); err != nil {
			return err
		}
		account, err = NewAccountWithPrivatekey(privateKey)
		if err != nil {
			return err
		}
	}

	encryptedSeed, err := crypto.AesEncrypt(seed, w.masterKey, w.iv)
	if err != nil {
		return err
	}
	contract, err := program.CreateSignatureProgramContext(account.PubKey())
	if err != nil {
		return err
	}
	err = w.SaveAccountData(account.ProgramHash.ToArray(), encryptedSeed, contract.ToArray())
	if err != nil {
		return err
	}
	w.account = account

	return nil
}

func (w *WalletImpl) GetDefaultAccount() (*Account, error) {
	if w.account == nil {
		return nil, errors.New("account error")
	}
	return w.account, nil
}

func (w *WalletImpl) GetAccount(pubKey *crypto.PubKey) (*Account, error) {
	redeemHash, err := program.CreateProgramHash(pubKey)
	if err != nil {
		return nil, fmt.Errorf("%v\n%s", err, "[Account] GetAccount redeemhash generated failed")
	}

	if redeemHash != w.account.ProgramHash {
		return nil, errors.New("invalid account")
	}

	return w.account, nil
}

func (w *WalletImpl) Sign(txn *transaction.Transaction) error {
	contract, err := w.GetContract()
	if err != nil {
		return fmt.Errorf("cannot get contract from wallet: %v", err)
	}

	account, err := w.GetDefaultAccount()
	if err != nil {
		return fmt.Errorf("no available account in wallet: %v", account)
	}

	signature, err := signature.SignBySigner(txn, account)
	if err != nil {
		return err
	}

	program := contract.NewProgram(signature)

	txn.SetPrograms([]*pb.Program{program})

	return nil
}

func verifyPasswordKey(passwordKey []byte, passwordHash []byte) bool {
	keyHash := sha256.Sum256(passwordKey)
	if !bytes.Equal(passwordHash, keyHash[:]) {
		fmt.Println("error: password wrong")
		return false
	}

	return true
}

func (w *WalletImpl) ChangePassword(oldPassword []byte, newPassword []byte) bool {
	// check original password
	oldPasswordKey := crypto.ToAesKey(oldPassword)
	passwordKeyHash, err := HexStringToBytes(w.Data.PasswordHash)
	if err != nil {
		return false
	}
	if ok := verifyPasswordKey(oldPasswordKey, passwordKeyHash); !ok {
		return false
	}

	// encrypt master key with new password
	newPasswordKey := crypto.ToAesKey(newPassword)
	newPasswordHash := sha256.Sum256(newPasswordKey)
	newMasterKey, err := crypto.AesEncrypt(w.masterKey, newPasswordKey, w.iv)
	if err != nil {
		fmt.Println("error: set new password failed")
		return false
	}

	// update wallet file
	err = w.SaveBasicData(WalletVersion, w.iv, newMasterKey, newPasswordHash[:])
	if err != nil {
		return false
	}

	return true
}

func (w *WalletImpl) GetContract() (*program.ProgramContext, error) {
	if w.contract == nil {
		return nil, errors.New("contract error")
	}

	return w.contract, nil
}

func GetWallet() (Wallet, error) {
	walletFileName := config.Parameters.WalletFile
	if !FileExisted(walletFileName) {
		serviceConfig.Status = serviceConfig.Status | serviceConfig.SERVICE_STATUS_NO_WALLET_FILE
		return nil, fmt.Errorf("wallet file %s does not exist, please create a wallet using nknc.", walletFileName)
	} else {
		serviceConfig.Status = serviceConfig.Status &^ serviceConfig.SERVICE_STATUS_NO_WALLET_FILE
	}

	passwd, err := password.GetAccountPassword()
	if err != nil {
		serviceConfig.Status = serviceConfig.Status | serviceConfig.SERVICE_STATUS_NO_PASSWORD
		return nil, fmt.Errorf("get password error: %v", err)
	} else {
		serviceConfig.Status = serviceConfig.Status &^ serviceConfig.SERVICE_STATUS_NO_PASSWORD
	}

	if !config.Parameters.AllowEmptyBeneficiaryAddress && config.Parameters.BeneficiaryAddr == "" {
		serviceConfig.Status = serviceConfig.Status | serviceConfig.SERVICE_STATUS_NO_BENEFICIARY
		return nil, fmt.Errorf("wait for set beneficiary address.")
	} else {
		serviceConfig.Status = serviceConfig.Status &^ serviceConfig.SERVICE_STATUS_NO_BENEFICIARY
	}

	w, err := OpenWallet(walletFileName, passwd)
	if err != nil {
		serviceConfig.Status = serviceConfig.Status | serviceConfig.SERVICE_STATUS_NO_PASSWORD
		return nil, fmt.Errorf("open wallet error: %v", err)
	} else {
		serviceConfig.Status = serviceConfig.Status &^ serviceConfig.SERVICE_STATUS_NO_PASSWORD
	}
	serviceConfig.Status = serviceConfig.SERVICE_STATUS_RUNNING
	return w, nil
}

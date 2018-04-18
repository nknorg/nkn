package wallet

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"

	"bytes"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/contract"
	ct "github.com/nknorg/nkn/core/contract"
	"github.com/nknorg/nkn/core/ledger"
	sig "github.com/nknorg/nkn/core/signature"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/password"
)

const (
	DefaultBookKeeperCount = 4
	WalletIVLength         = 16
	WalletMasterKeyLength  = 32
	WalletFileName         = "wallet.dat"
)

type Wallet interface {
	Sign(context *ct.ContractContext) error
	GetAccount(pubKey *crypto.PubKey) (*Account, error)
	GetDefaultAccount() (*Account, error)
	GetUnspent() (map[Uint256][]*transaction.UTXOUnspent, error)
}

type WalletImpl struct {
	path      string
	iv        []byte
	masterKey []byte
	account   *Account
	contract  *ct.Contract
	*Store
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
	err = store.SaveBasicData([]byte(WalletStoreVersion), iv, encryptedMasterKey, pwdhash[:])
	if err != nil {
		return nil, err
	}

	w := &WalletImpl{
		path:      path,
		iv:        iv,
		masterKey: masterKey,
		Store:     store,
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

	encryptedPrivateKey, err := HexStringToBytes(store.Data.AccountData.PrivateKeyEncrypted)
	if err != nil {
		return nil, err
	}
	privateKey, err := crypto.AesDecrypt(encryptedPrivateKey, masterKey, iv)
	if err != nil {
		return nil, err
	}
	account, err := NewAccountWithPrivatekey(privateKey)
	if err != nil {
		return nil, err
	}

	rawdata, _ := HexStringToBytes(store.Data.ContractData)
	r := bytes.NewReader(rawdata)
	ct := new(ct.Contract)
	ct.Deserialize(r)

	return &WalletImpl{
		path:      path,
		iv:        iv,
		masterKey: masterKey,
		account:   account,
		contract:  ct,
		Store:     store,
	}, nil
}

func RecoverWallet(path string, password []byte, privateKeyHex string) (*WalletImpl, error) {
	wallet, err := NewWallet(path, password, false)
	if err != nil {
		return nil, errors.New("create new wallet error")
	}
	privateKey, err := HexStringToBytes(privateKeyHex)
	if err != nil {
		return nil, err
	}
	err = wallet.CreateAccount(privateKey)
	if err != nil {
		return nil, err
	}

	return wallet, nil
}

func (p *WalletImpl) CreateAccount(privateKey []byte) error {
	var account *Account
	var err error
	if privateKey == nil {
		account, err = NewAccount()
	} else {
		account, err = NewAccountWithPrivatekey(privateKey)
	}
	if err != nil {
		return err
	}
	encryptedPrivateKey, err := crypto.AesEncrypt(account.PrivateKey, p.masterKey, p.iv)
	if err != nil {
		return err
	}
	contract, err := contract.CreateSignatureContract(account.PubKey())
	if err != nil {
		return err
	}
	err = p.SaveAccountData(account.ProgramHash.ToArray(), encryptedPrivateKey, contract.ToArray())
	if err != nil {
		return err
	}
	p.account = account

	return nil
}

func (cl *WalletImpl) GetDefaultAccount() (*Account, error) {
	if cl.account == nil {
		return nil, errors.New("account error")
	}
	return cl.account, nil
}

func (cl *WalletImpl) GetAccount(pubKey *crypto.PubKey) (*Account, error) {
	signatureRedeemScript, err := contract.CreateSignatureRedeemScript(pubKey)
	if err != nil {
		return nil, err
	}
	programHash, err := ToCodeHash(signatureRedeemScript)
	if err != nil {
		return nil, err
	}

	if programHash != cl.account.ProgramHash {
		return nil, errors.New("invalid account")
	}

	return cl.account, nil
}

func (cl *WalletImpl) Sign(context *ct.ContractContext) error {
	var err error
	contract, err := cl.GetContract()
	if err != nil {
		return errors.New("no available contract in wallet")
	}
	account, err := cl.GetDefaultAccount()
	if err != nil {
		return errors.New("no available account in wallet")
	}

	signature, err := sig.SignBySigner(context.Data, account)
	if err != nil {
		return err
	}
	err = context.AddContract(contract, account.PublicKey, signature)
	if err != nil {
		return err
	}

	return nil
}

func verifyPasswordKey(passwordKey []byte, passwordHash []byte) bool {
	keyHash := sha256.Sum256(passwordKey)
	if !IsEqualBytes(passwordHash, keyHash[:]) {
		fmt.Println("error: password wrong")
		return false
	}

	return true
}

func (cl *WalletImpl) ChangePassword(oldPassword []byte, newPassword []byte) bool {
	// check original password
	oldPasswordKey := crypto.ToAesKey(oldPassword)
	passwordKeyHash, err := HexStringToBytes(cl.Data.PasswordHash)
	if err != nil {
		return false
	}
	if ok := verifyPasswordKey(oldPasswordKey, passwordKeyHash); !ok {
		return false
	}

	// encrypt master key with new password
	newPasswordKey := crypto.ToAesKey(newPassword)
	newPasswordHash := sha256.Sum256(newPasswordKey)
	newMasterKey, err := crypto.AesEncrypt(cl.masterKey, newPasswordKey, cl.iv)
	if err != nil {
		fmt.Println("error: set new password failed")
		return false
	}

	// update wallet file
	err = cl.SaveBasicData([]byte(WalletStoreVersion), cl.iv, newMasterKey, newPasswordHash[:])
	if err != nil {
		return false
	}

	return true
}

func (cl *WalletImpl) GetContract() (*ct.Contract, error) {
	if cl.contract == nil {
		return nil, errors.New("contract error")
	}

	return cl.contract, nil
}

func (cl *WalletImpl) GetUnspent() (map[Uint256][]*transaction.UTXOUnspent, error) {
	account, err := cl.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	ret, err := ledger.DefaultLedger.Store.GetUnspentsFromProgramHash(account.ProgramHash)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func GetWallet() Wallet {
	if !FileExisted(WalletFileName) {
		log.Fatal(fmt.Sprintf("No %s detected, please create a wallet by using command line.", WalletFileName))
		os.Exit(1)
	}
	passwd, err := password.GetAccountPassword()
	if err != nil {
		log.Fatal("Get password error.")
		os.Exit(1)
	}
	c, err := OpenWallet(WalletFileName, passwd)
	if err != nil {
		return nil
	}
	return c
}

func GetBookKeepers() []*crypto.PubKey {
	var pubKeys = []*crypto.PubKey{}
	sort.Strings(config.Parameters.BookKeepers)
	for _, key := range config.Parameters.BookKeepers {
		pubKey := []byte(key)
		pubKey, err := hex.DecodeString(key)
		// TODO Convert the key string to byte
		k, err := crypto.DecodePoint(pubKey)
		if err != nil {
			log.Error("Incorrectly book keepers key")
			return nil
		}
		pubKeys = append(pubKeys, k)
	}

	return pubKeys
}

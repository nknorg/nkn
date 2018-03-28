package wallet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sort"
	. "nkn-core/common"
	"nkn-core/common/config"
	"nkn-core/common/log"
	"nkn-core/common/password"
	"nkn-core/core/contract"
	ct "nkn-core/core/contract"
	"nkn-core/core/ledger"
	sig "nkn-core/core/signature"
	"nkn-core/core/transaction"
	"nkn-core/crypto"
	. "nkn-core/errors"
)

const (
	DefaultBookKeeperCount = 4
	WalletFileName         = "wallet.dat"
)

type Wallet interface {
	Sign(context *ct.ContractContext) error

	ContainsAccount(pubKey *crypto.PubKey) bool
	CreateAccount() (*Account, error)
	GetDefaultAccount() (*Account, error)
	GetAccount(pubKey *crypto.PubKey) (*Account, error)

	CreateContract(account *Account) (*ct.Contract, error)
	GetContract() (*ct.Contract, error)

	GetUnspent() (map[Uint256][]*transaction.UTXOUnspent, error)
}

type WalletImpl struct {
	path      string
	iv        []byte
	masterKey []byte

	account  *Account
	contract *ct.Contract

	store
}

func Create(path string, passwordKey []byte) (*WalletImpl, error) {
	client := NewWallet(path, passwordKey, true)
	if client == nil {
		return nil, errors.New("client nil")
	}
	account, err := client.CreateAccount()
	if err != nil {
		return nil, err
	}
	if _, err := client.CreateContract(account); err != nil {
		return nil, err
	}

	return client, nil
}

func Open(path string, passwordKey []byte) (*WalletImpl, error) {
	client := NewWallet(path, passwordKey, false)
	if client == nil {
		return nil, errors.New("client nil")
	}
	if err := client.LoadAccount(); err != nil {
		return nil, errors.New("load account error")
	}
	if err := client.LoadContract(); err != nil {
		return nil, errors.New("load contract error")
	}

	return client, nil
}

func Recover(path string, password []byte, privateKeyHex string) (*WalletImpl, error) {
	client := NewWallet(path, password, true)
	if client == nil {
		return nil, errors.New("client nil")
	}
	privateKeyBytes, err := HexStringToBytes(privateKeyHex)
	if err != nil {
		return nil, err
	}
	account, err := client.CreateAccountByPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, err
	}
	if _, err := client.CreateContract(account); err != nil {
		return nil, err
	}

	return client, nil
}

func NewWallet(path string, password []byte, create bool) *WalletImpl {
	client := &WalletImpl{
		path:     path,
		account:  new(Account),
		contract: new(ct.Contract),
		store:    store{path: path},
	}

	passwordKey := crypto.ToAesKey(password)
	if create {
		//create new client
		client.iv = make([]byte, 16)
		client.masterKey = make([]byte, 32)

		//generate random number for iv/masterkey
		for i := 0; i < 16; i++ {
			client.iv[i] = byte(rand.Intn(256))
		}
		for i := 0; i < 32; i++ {
			client.masterKey[i] = byte(rand.Intn(256))
		}

		//new client store (build DB)
		client.BuildDatabase(path)

		if err := client.SaveStoredData("Version", []byte(WalletStoreVersion)); err != nil {
			log.Error(err)
			return nil
		}

		pwdhash := sha256.Sum256(passwordKey)
		if err := client.SaveStoredData("PasswordHash", pwdhash[:]); err != nil {
			log.Error(err)
			return nil
		}
		if err := client.SaveStoredData("IV", client.iv[:]); err != nil {
			log.Error(err)
			return nil
		}

		aesmk, err := crypto.AesEncrypt(client.masterKey[:], passwordKey, client.iv)
		if err != nil {
			log.Error(err)
			return nil
		}
		if err := client.SaveStoredData("MasterKey", aesmk); err != nil {
			log.Error(err)
			return nil
		}
	} else {
		if ok := client.verifyPasswordKey(passwordKey); !ok {
			return nil
		}
		var err error
		client.iv, err = client.LoadStoredData("IV")
		if err != nil {
			fmt.Println("error: failed to load iv")
			return nil
		}
		encryptedMasterKey, err := client.LoadStoredData("MasterKey")
		if err != nil {
			fmt.Println("error: failed to load master key")
			return nil
		}
		client.masterKey, err = crypto.AesDecrypt(encryptedMasterKey, passwordKey, client.iv)
		if err != nil {
			fmt.Println("error: failed to decrypt master key")
			return nil
		}
	}
	ClearBytes(passwordKey, len(passwordKey))

	return client
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

func (cl *WalletImpl) ChangePassword(oldPassword []byte, newPassword []byte) bool {
	// check password
	oldPasswordKey := crypto.ToAesKey(oldPassword)
	if !cl.verifyPasswordKey(oldPasswordKey) {
		fmt.Println("error: password verification failed")
		return false
	}

	// encrypt master key with new password
	newPasswordKey := crypto.ToAesKey(newPassword)
	newMasterKey, err := crypto.AesEncrypt(cl.masterKey, newPasswordKey, cl.iv)
	if err != nil {
		fmt.Println("error: set new password failed")
		return false
	}

	// update wallet file
	newPasswordHash := sha256.Sum256(newPasswordKey)
	if err := cl.SaveStoredData("PasswordHash", newPasswordHash[:]); err != nil {
		fmt.Println("error: wallet update failed(password hash)")
		return false
	}
	if err := cl.SaveStoredData("MasterKey", newMasterKey); err != nil {
		fmt.Println("error: wallet update failed (encrypted master key)")
		return false
	}
	ClearBytes(newPasswordKey, len(newPasswordKey))

	return true
}

func (cl *WalletImpl) ContainsAccount(pubKey *crypto.PubKey) bool {
	signatureRedeemScript, err := contract.CreateSignatureRedeemScript(pubKey)
	if err != nil {
		return false
	}
	programHash, err := ToCodeHash(signatureRedeemScript)
	if err != nil {
		return false
	}

	return cl.account.ProgramHash == programHash
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

func (cl *WalletImpl) verifyPasswordKey(passwordKey []byte) bool {
	savedPasswordHash, err := cl.LoadStoredData("PasswordHash")
	if err != nil {
		fmt.Println("error: failed to load password hash")
		return false
	}
	if savedPasswordHash == nil {
		fmt.Println("error: saved password hash is nil")
		return false
	}
	passwordHash := sha256.Sum256(passwordKey)
	if !IsEqualBytes(savedPasswordHash, passwordHash[:]) {
		fmt.Println("error: password wrong")
		return false
	}
	return true
}

func (cl *WalletImpl) EncryptPrivateKey(prikey []byte) ([]byte, error) {
	enc, err := crypto.AesEncrypt(prikey, cl.masterKey, cl.iv)
	if err != nil {
		return nil, err
	}

	return enc, nil
}

func (cl *WalletImpl) DecryptPrivateKey(prikey []byte) ([]byte, error) {
	if prikey == nil {
		return nil, NewDetailErr(errors.New("The PriKey is nil"), ErrNoCode, "")
	}
	if len(prikey) != 96 {
		return nil, NewDetailErr(errors.New("The len of PriKeyEnc is not 96bytes"), ErrNoCode, "")
	}

	dec, err := crypto.AesDecrypt(prikey, cl.masterKey, cl.iv)
	if err != nil {
		return nil, err
	}

	return dec, nil
}

// CreateAccount create a new Account then save it
func (cl *WalletImpl) CreateAccount() (*Account, error) {
	account, err := NewAccount()
	if err != nil {
		return nil, err
	}
	if err := cl.SaveAccount(account); err != nil {
		return nil, err
	}

	return account, nil
}

func (cl *WalletImpl) CreateAccountByPrivateKey(privateKey []byte) (*Account, error) {
	account, err := NewAccountWithPrivatekey(privateKey)
	if err != nil {
		return nil, err
	}

	if err := cl.SaveAccount(account); err != nil {
		return nil, err
	}

	return account, nil
}

// SaveAccount saves a Account to memory and db
func (cl *WalletImpl) SaveAccount(ac *Account) error {
	programHash := ac.ProgramHash
	cl.account = ac

	decryptedPrivateKey := make([]byte, 96)
	temp, err := ac.PublicKey.EncodePoint(false)
	if err != nil {
		return err
	}
	for i := 1; i <= 64; i++ {
		decryptedPrivateKey[i-1] = temp[i]
	}
	for i := len(ac.PrivateKey) - 1; i >= 0; i-- {
		decryptedPrivateKey[96+i-len(ac.PrivateKey)] = ac.PrivateKey[i]
	}
	encryptedPrivateKey, err := cl.EncryptPrivateKey(decryptedPrivateKey)
	if err != nil {
		return err
	}
	ClearBytes(decryptedPrivateKey, 96)

	// save Account keys to db
	err = cl.SaveAccountData(programHash.ToArray(), encryptedPrivateKey)
	if err != nil {
		return err
	}

	return nil
}

// LoadAccounts loads all accounts from db to memory
func (cl *WalletImpl) LoadAccount() error {
	account, err := cl.LoadAccountData()
	if err != nil {
		return err
	}
	encryptedKeyPair, err := HexStringToBytes(account.PrivateKeyEncrypted)
	if err != nil {
		return err
	}
	keyPair, err := cl.DecryptPrivateKey(encryptedKeyPair)
	if err != nil {
		return err
	}
	privateKey := keyPair[64:96]
	ac, err := NewAccountWithPrivatekey(privateKey)
	if err != nil {
		return err
	}

	cl.account = ac

	return nil
}

// CreateContract creates a singlesig contract to wallet
func (cl *WalletImpl) CreateContract(account *Account) (*contract.Contract, error) {
	contract, err := contract.CreateSignatureContract(account.PubKey())
	if err != nil {
		return nil, err
	}
	if err := cl.SaveContract(contract); err != nil {
		return nil, err
	}

	return contract, nil
}

// SaveContract saves a contract to memory and db
func (cl *WalletImpl) SaveContract(ct *contract.Contract) error {

	// save contract to memory
	cl.contract = ct

	// save contract to db
	return cl.SaveContractData(ct)
}

// LoadContracts loads all contracts from db to memory
func (cl *WalletImpl) LoadContract() error {
	contract, err := cl.LoadContractData()
	if err != nil {
		return err
	}
	rawdata, _ := HexStringToBytes(contract.RawData)
	r := bytes.NewReader(rawdata)
	ct := new(ct.Contract)
	ct.Deserialize(r)

	cl.contract = ct

	return nil
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

func GetClient() Wallet {
	if !FileExisted(WalletFileName) {
		log.Fatal(fmt.Sprintf("No %s detected, please create a wallet by using command line.", WalletFileName))
		os.Exit(1)
	}
	passwd, err := password.GetAccountPassword()
	if err != nil {
		log.Fatal("Get password error.")
		os.Exit(1)
	}
	c, err := Open(WalletFileName, passwd)
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

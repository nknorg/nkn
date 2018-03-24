package account

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"io"
	"sort"
	"sync"
	"time"

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
	"nkn-core/common/serialization"
	. "nkn-core/errors"
)

const (
	DefaultBookKeeperCount = 4
	WalletFileName         = "wallet.dat"
	MAINACCOUNT            = "main-account"
	SUBACCOUNT             = "sub-account"
)

type Client interface {
	Sign(context *ct.ContractContext) error

	ContainsAccount(pubKey *crypto.PubKey) bool
	CreateAccount() (*Account, error)
	DeleteAccount(programHash Uint160) error
	GetAccount(pubKey *crypto.PubKey) (*Account, error)
	GetDefaultAccount() (*Account, error)
	GetAccountByProgramHash(programHash Uint160) *Account
	GetAccounts() []*Account

	CreateContract(account *Account) error
	CreateMultiSignContract(contractOwner Uint160, m int, publicKeys []*crypto.PubKey) error
	GetContracts() []*ct.Contract
	DeleteContract(programHash Uint160) error

	GetCoins() map[*transaction.UTXOTxInput]*Coin
	DeleteCoinsData(programHash Uint160) error
}

type ClientImpl struct {
	mu sync.Mutex

	path      string
	iv        []byte
	masterKey []byte

	mainAccount Uint160
	accounts    map[Uint160]*Account
	contracts   map[Uint160]*ct.Contract
	coins       map[*transaction.UTXOTxInput]*Coin

	watchOnly     []Uint160
	currentHeight uint32

	FileStore
	isRunning bool
}

func Create(path string, passwordKey []byte) (*ClientImpl, error) {
	client := NewClient(path, passwordKey, true)
	if client == nil {
		return nil, errors.New("client nil")
	}
	account, err := client.CreateAccount()
	if err != nil {
		return nil, err
	}
	if err := client.CreateContract(account); err != nil {
		return nil, err
	}
	client.mainAccount = account.ProgramHash

	return client, nil
}

func Open(path string, passwordKey []byte) (*ClientImpl, error) {
	client := NewClient(path, passwordKey, false)
	if client == nil {
		return nil, errors.New("client nil")
	}
	if err := client.LoadAccounts(); err != nil {
		return nil, errors.New("Load accounts failure")
	}
	if err := client.LoadContracts(); err != nil {
		return nil, errors.New("Load contracts failure")
	}
	if err := client.LoadCoins(); err != nil {
		return nil, errors.New("Load coins failure")
	}

	return client, nil
}

func Recover(path string, password []byte, privateKeyHex string) (*ClientImpl, error) {
	client := NewClient(path, password, true)
	if client == nil {
		return nil, errors.New("client nil")
	}

	privateKeyBytes, err := HexStringToBytes(privateKeyHex)
	if err != nil {
		return nil, err
	}

	// recover Account
	account, err := client.CreateAccountByPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, err
	}

	// recover contract
	if err := client.CreateContract(account); err != nil {
		return nil, err
	}

	return client, nil
}

func (client *ClientImpl) ProcessBlocks() {
	time.Sleep(time.Second)
	for client.isRunning {
		for true {
			blockHeight := ledger.DefaultLedger.GetLocalBlockChainHeight()
			if client.currentHeight > blockHeight {
				break
			}
			block, err := ledger.DefaultLedger.GetBlockWithHeight(client.currentHeight)
			if err != nil {
				fmt.Fprintf(os.Stderr, "fatal error: syncing failed, block missing, height %d\n", client.currentHeight)
				break
			}
			client.ProcessOneBlock(block)
		}
		time.Sleep(6 * time.Second)
	}
}

func (client *ClientImpl) ProcessOneBlock(block *ledger.Block) {
	client.mu.Lock()
	defer client.mu.Unlock()

	var needUpdate bool
	// received coins
	for _, tx := range block.Transactions {
		for index, output := range tx.Outputs {
			if contract, ok := client.contracts[output.ProgramHash]; ok {
				input := &transaction.UTXOTxInput{ReferTxID: tx.Hash(), ReferTxOutputIndex: uint16(index)}
				if _, ok := client.coins[input]; !ok {
					var newCoin Coin
					switch {
					case contract.IsStandard():
						newCoin = Coin{Output: output, AddressType: SingleSign}
					}
					client.coins[input] = &newCoin
					needUpdate = true
				}
			}
		}
	}

	for _, tx := range block.Transactions {
		for _, input := range tx.UTXOInputs {
			for k := range client.coins {
				if k.ReferTxOutputIndex == input.ReferTxOutputIndex && k.ReferTxID == input.ReferTxID {
					delete(client.coins, k)
					needUpdate = true
				}
			}
		}
	}

	if needUpdate {
		if err := client.SaveCoins(); err != nil {
			fmt.Fprintf(os.Stderr, "saving coins error: %v\n", err)
		}
	}

	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.LittleEndian, &client.currentHeight)
	client.SaveStoredData("Height", bytesBuffer.Bytes())
	client.currentHeight++
}

func NewClient(path string, password []byte, create bool) *ClientImpl {
	client := &ClientImpl{
		path:          path,
		accounts:      map[Uint160]*Account{},
		contracts:     map[Uint160]*ct.Contract{},
		coins:         map[*transaction.UTXOTxInput]*Coin{},
		currentHeight: 0,
		FileStore:     FileStore{path: path},
		isRunning:     true,
	}

	passwordKey := crypto.ToAesKey(password)
	if create {
		//create new client
		client.iv = make([]byte, 16)
		client.masterKey = make([]byte, 32)
		client.watchOnly = []Uint160{}

		//generate random number for iv/masterkey
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for i := 0; i < 16; i++ {
			client.iv[i] = byte(r.Intn(256))
		}
		for i := 0; i < 32; i++ {
			client.masterKey[i] = byte(r.Intn(256))
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

		// if has local blockchain database, then update wallet block height. Otherwise, wallet block height is 0 by default
		if ledger.DefaultLedger != nil && ledger.DefaultLedger.Blockchain != nil {
			client.currentHeight = ledger.DefaultLedger.GetLocalBlockChainHeight()
			bytesBuffer := bytes.NewBuffer([]byte{})
			binary.Write(bytesBuffer, binary.LittleEndian, &client.currentHeight)
			if err := client.SaveStoredData("Height", bytesBuffer.Bytes()); err != nil {
				return nil
			}
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
		tmp, err := client.LoadStoredData("Height")
		if err != nil {
			return nil
		}
		bytesBuffer := bytes.NewBuffer(tmp)
		var height uint32
		binary.Read(bytesBuffer, binary.LittleEndian, &height)
		client.currentHeight = height
	}
	ClearBytes(passwordKey, len(passwordKey))
	// if has local blockchain database and running flag is set, then sync wallet data
	if ledger.DefaultLedger != nil && ledger.DefaultLedger.Blockchain != nil && client.isRunning {
		go client.ProcessBlocks()
	}

	return client
}

func (cl *ClientImpl) GetDefaultAccount() (*Account, error) {
	return cl.GetAccountByProgramHash(cl.mainAccount), nil
}

func (cl *ClientImpl) GetAccount(pubKey *crypto.PubKey) (*Account, error) {
	signatureRedeemScript, err := contract.CreateSignatureRedeemScript(pubKey)
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "CreateSignatureRedeemScript failed")
	}
	programHash, err := ToCodeHash(signatureRedeemScript)
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "ToCodeHash failed")
	}
	return cl.GetAccountByProgramHash(programHash), nil
}

func (cl *ClientImpl) GetAccountByProgramHash(programHash Uint160) *Account {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if account, ok := cl.accounts[programHash]; ok {
		return account
	}
	return nil
}

func (cl *ClientImpl) GetContract(programHash Uint160) *ct.Contract {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if contract, ok := cl.contracts[programHash]; ok {
		return contract
	}
	return nil
}

func (cl *ClientImpl) ChangePassword(oldPassword []byte, newPassword []byte) bool {
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

func (cl *ClientImpl) ContainsAccount(pubKey *crypto.PubKey) bool {
	signatureRedeemScript, err := contract.CreateSignatureRedeemScript(pubKey)
	if err != nil {
		return false
	}
	programHash, err := ToCodeHash(signatureRedeemScript)
	if err != nil {
		return false
	}
	if cl.GetAccountByProgramHash(programHash) != nil {
		return true
	} else {
		return false
	}
}

func (cl *ClientImpl) Sign(context *ct.ContractContext) error {
	for _, hash := range context.ProgramHashes {
		contract := cl.GetContract(hash)
		if contract == nil {
			return errors.New("no available contract in wallet")
		}
		switch {
		case contract.IsStandard():
			acct := cl.GetAccountByProgramHash(hash)
			if acct == nil {
				return errors.New("no available account in wallet to do single-sign")
			}
			signature, err := sig.SignBySigner(context.Data, acct)
			if err != nil {
				return err
			}
			if err := context.AddContract(contract, acct.PublicKey, signature); err != nil {
				return err
			}
		}
	}

	return nil
}

func (cl *ClientImpl) verifyPasswordKey(passwordKey []byte) bool {
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
	///ClearBytes(passwordKey, len(passwordKey))
	if !IsEqualBytes(savedPasswordHash, passwordHash[:]) {
		fmt.Println("error: password wrong")
		return false
	}
	return true
}

func (cl *ClientImpl) EncryptPrivateKey(prikey []byte) ([]byte, error) {
	enc, err := crypto.AesEncrypt(prikey, cl.masterKey, cl.iv)
	if err != nil {
		return nil, err
	}

	return enc, nil
}

func (cl *ClientImpl) DecryptPrivateKey(prikey []byte) ([]byte, error) {
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
func (cl *ClientImpl) CreateAccount() (*Account, error) {
	account, err := NewAccount()
	if err != nil {
		return nil, err
	}
	if err := cl.SaveAccount(account); err != nil {
		return nil, err
	}

	return account, nil
}

func (cl *ClientImpl) DeleteAccount(programHash Uint160) error {
	// remove from memory
	delete(cl.accounts, programHash)
	// remove from db
	return cl.DeleteAccountData(BytesToHexString(programHash.ToArray()))
}

func (cl *ClientImpl) CreateAccountByPrivateKey(privateKey []byte) (*Account, error) {
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
func (cl *ClientImpl) SaveAccount(ac *Account) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	// save Account to memory
	programHash := ac.ProgramHash
	cl.accounts[programHash] = ac

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
func (cl *ClientImpl) LoadAccounts() error {
	accounts := map[Uint160]*Account{}

	account, err := cl.LoadAccountData()
	if err != nil {
		return err
	}
	for _, a := range account {
		if a.Type == MAINACCOUNT {
			p, _ := HexStringToBytes(a.ProgramHash)
			cl.mainAccount, _ = Uint160ParseFromBytes(p)
		}
		encryptedKeyPair, _ := HexStringToBytes(a.PrivateKeyEncrypted)
		keyPair, err := cl.DecryptPrivateKey(encryptedKeyPair)
		if err != nil {
			log.Error(err)
			continue
		}
		privateKey := keyPair[64:96]
		ac, err := NewAccountWithPrivatekey(privateKey)
		accounts[ac.ProgramHash] = ac
	}

	cl.accounts = accounts
	return nil
}

// CreateContract creates a singlesig contract to wallet
func (cl *ClientImpl) CreateContract(account *Account) error {
	contract, err := contract.CreateSignatureContract(account.PubKey())
	if err != nil {
		return err
	}
	if err := cl.SaveContract(contract); err != nil {
		return err
	}
	return nil
}

// CreateMultiSignContract creates a multisig contract to wallet
func (cl *ClientImpl) CreateMultiSignContract(contractOwner Uint160, m int, publicKeys []*crypto.PubKey) error {
	contract, err := contract.CreateMultiSigContract(contractOwner, m, publicKeys)
	if err != nil {
		return err
	}
	if err := cl.SaveContract(contract); err != nil {
		return err
	}
	return nil
}

func (cl *ClientImpl) DeleteContract(programHash Uint160) error {
	delete(cl.contracts, programHash)
	return cl.DeleteContractData(BytesToHexString(programHash.ToArray()))
}

// SaveContract saves a contract to memory and db
func (cl *ClientImpl) SaveContract(ct *contract.Contract) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	// save contract to memory
	cl.contracts[ct.ProgramHash] = ct

	// save contract to db
	return cl.SaveContractData(ct)
}

// LoadContracts loads all contracts from db to memory
func (cl *ClientImpl) LoadContracts() error {
	contracts := map[Uint160]*ct.Contract{}

	contract, err := cl.LoadContractData()
	if err != nil {
		return err
	}
	for _, c := range contract {
		rawdata, _ := HexStringToBytes(c.RawData)
		rdreader := bytes.NewReader(rawdata)
		ct := new(ct.Contract)
		ct.Deserialize(rdreader)

		programHash, _ := HexStringToBytes(c.ProgramHash)
		programhash, _ := Uint160ParseFromBytes(programHash)
		ct.ProgramHash = programhash
		contracts[ct.ProgramHash] = ct
	}

	cl.contracts = contracts
	return nil
}

// GetContracts returns all contracts in wallet
func (client *ClientImpl) GetContracts() []*ct.Contract {
	client.mu.Lock()
	defer client.mu.Unlock()

	contracts := []*ct.Contract{}
	for _, v := range client.contracts {
		contracts = append(contracts, v)
	}
	return contracts
}

func (client *ClientImpl) LoadCoins() error {
	loadedCoin, err := client.LoadCoinsData()
	if err != nil {
		return err
	}
	for input, coin := range loadedCoin {
		client.coins[input] = coin
	}
	return nil
}
func (client *ClientImpl) SaveCoins() error {
	if err := client.SaveCoinsData(client.coins); err != nil {
		return err
	}
	return nil
}

func (client *ClientImpl) DeleteCoins() error {
	for in, coin := range client.coins {
		// remove from memory
		delete(client.coins, in)
		// remove from db
		if err := client.DeleteCoinsData(coin.Output.ProgramHash); err != nil {
			fmt.Fprintf(os.Stderr, "delete coin error: %v\n", err)
			continue
		}
	}
	return nil
}

func GetClient() Client {
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

func (client *ClientImpl) GetCoins() map[*transaction.UTXOTxInput]*Coin {
	client.mu.Lock()
	defer client.mu.Unlock()

	return client.coins
}

func (client *ClientImpl) GetAccounts() []*Account {
	client.mu.Lock()
	defer client.mu.Unlock()

	accounts := []*Account{}
	for _, v := range client.accounts {
		accounts = append(accounts, v)
	}
	return accounts
}

func (client *ClientImpl) Rebuild() error {
	// reset wallet block height
	client.currentHeight = 0
	var height uint32 = 0
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.LittleEndian, &height)
	if err := client.SaveStoredData("Height", bytesBuffer.Bytes()); err != nil {
		return err
	}

	// reset coins
	if err := client.DeleteCoins(); err != nil {
		return err
	}

	return nil
}

type AddressType byte

const (
	SingleSign AddressType = 0
)

type Coin struct {
	Output      *transaction.TxOutput
	AddressType AddressType
}

func (coin *Coin) Serialize(w io.Writer, version string) error {
	coin.Output.Serialize(w)
	switch {
	case version > "1.0.0":
		w.Write([]byte{byte(coin.AddressType)})
	default:
		break
	}
	return nil
}

func (coin *Coin) Deserialize(r io.Reader, version string) error {
	coin.Output = new(transaction.TxOutput)
	coin.Output.Deserialize(r)
	switch {
	case version > "1.0.0":
		addrType, err := serialization.ReadUint8(r)
		if err != nil {
			return err
		}
		coin.AddressType = AddressType(addrType)
	default:
		break
	}

	return nil
}

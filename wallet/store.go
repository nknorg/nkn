package wallet

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"sync"

	. "github.com/nknorg/nkn/common"
)

const (
	WalletStoreVersion = "0.0.1"
)

type HeaderData struct {
	PasswordHash string
	IV           string
	MasterKey    string
	Version      string
}

type AccountData struct {
	Address             string
	ProgramHash         string
	PrivateKeyEncrypted string
	ContractData        string
}

type WalletData struct {
	HeaderData
	AccountData
}

type Store struct {
	sync.RWMutex

	Path string
	Data WalletData
}

func NewStore(fullPath string) (*Store, error) {
	if FileExisted(fullPath) {
		return nil, errors.New("wallet store exists")
	}

	var walletData WalletData
	jsonBlob, err := json.Marshal(walletData)
	if err != nil {
		return nil, err
	}
	_, name := path.Split(fullPath)
	err = ioutil.WriteFile(name, jsonBlob, 0666)
	if err != nil {
		return nil, err
	}

	return &Store{
		Path: fullPath,
		Data: walletData,
	}, nil
}

func LoadStore(fullPath string) (*Store, error) {
	if !FileExisted(fullPath) {
		return nil, errors.New("wallet store doesn't exists")
	}
	fileData, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	var walletData WalletData
	err = json.Unmarshal(fileData, &walletData)
	if err != nil {
		return nil, err
	}
	return &Store{
		Path: fullPath,
		Data: walletData,
	}, nil
}

func (p *Store) read() ([]byte, error) {
	p.RLock()
	defer p.RUnlock()

	return ioutil.ReadFile(p.Path)
}

func (p *Store) write(data []byte) error {
	p.Lock()
	defer p.Unlock()

	dir, name := path.Split(p.Path)
	f, err := ioutil.TempFile(dir, name)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if permErr := os.Chmod(f.Name(), 0666); err == nil {
		err = permErr
	}
	if err == nil {
		err = os.Rename(f.Name(), name)
	}
	if err != nil {
		os.Remove(f.Name())
	}

	return err
}

func (p *Store) SaveAccountData(programHash []byte, encryptedPrivateKey []byte, contract []byte) error {
	oldBlob, err := p.read()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(oldBlob, &p.Data); err != nil {
		return err
	}
	pHash, err := Uint160ParseFromBytes(programHash)
	if err != nil {
		return err
	}
	addr, err := pHash.ToAddress()
	if err != nil {
		return err
	}
	p.Data.AccountData = AccountData{
		Address:             addr,
		ProgramHash:         BytesToHexString(programHash),
		PrivateKeyEncrypted: BytesToHexString(encryptedPrivateKey),
		ContractData:        BytesToHexString(contract),
	}
	newBlob, err := json.Marshal(p.Data)
	if err != nil {
		return err
	}
	err = p.write(newBlob)
	if err != nil {
		return err
	}

	return nil
}

func (p *Store) SaveBasicData(version, iv, masterKey, passwordHash []byte) error {
	oldBlob, err := p.read()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(oldBlob, &p.Data); err != nil {
		return err
	}

	p.Data.Version = string(version)
	p.Data.IV = BytesToHexString(iv)
	p.Data.MasterKey = BytesToHexString(masterKey)
	p.Data.PasswordHash = BytesToHexString(passwordHash)

	newBlob, err := json.Marshal(p.Data)
	if err != nil {
		return err
	}
	err = p.write(newBlob)
	if err != nil {
		return err
	}

	return nil
}

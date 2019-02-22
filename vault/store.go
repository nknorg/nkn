package vault

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"runtime"
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

type WalletStore struct {
	sync.RWMutex

	Path string
	Data WalletData
}

func NewStore(fullPath string) (*WalletStore, error) {
	if FileExisted(fullPath) {
		return nil, errors.New("wallet store exists")
	}

	var walletData WalletData
	jsonBlob, err := json.Marshal(walletData)
	if err != nil {
		return nil, err
	}

	name := fullPath
	err = ioutil.WriteFile(name, jsonBlob, 0666)
	if err != nil {
		return nil, err
	}

	return &WalletStore{
		Path: fullPath,
		Data: walletData,
	}, nil
}

func LoadStore(fullPath string) (*WalletStore, error) {
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
	return &WalletStore{
		Path: fullPath,
		Data: walletData,
	}, nil
}

func (s *WalletStore) read() ([]byte, error) {
	s.RLock()
	defer s.RUnlock()

	return ioutil.ReadFile(s.Path)
}

func (s *WalletStore) write(data []byte) error {
	s.Lock()
	defer s.Unlock()

	dir, name := path.Split(s.Path)
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
		if runtime.GOOS == "windows" {
			err = exec.Command("cmd", "/c", "move", "/Y", f.Name(), s.Path).Run()
		} else {
			err = exec.Command("mv", "-f", f.Name(), s.Path).Run()
		}
	}
	if err != nil {
		os.Remove(f.Name())
	}

	return err
}

func (s *WalletStore) SaveAccountData(programHash []byte, encryptedPrivateKey []byte, contract []byte) error {
	oldBlob, err := s.read()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(oldBlob, &s.Data); err != nil {
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
	s.Data.AccountData = AccountData{
		Address:             addr,
		ProgramHash:         BytesToHexString(programHash),
		PrivateKeyEncrypted: BytesToHexString(encryptedPrivateKey),
		ContractData:        BytesToHexString(contract),
	}
	newBlob, err := json.Marshal(s.Data)
	if err != nil {
		return err
	}
	err = s.write(newBlob)
	if err != nil {
		return err
	}

	return nil
}

func (s *WalletStore) SaveBasicData(version, iv, masterKey, passwordHash []byte) error {
	oldBlob, err := s.read()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(oldBlob, &s.Data); err != nil {
		return err
	}

	s.Data.Version = string(version)
	s.Data.IV = BytesToHexString(iv)
	s.Data.MasterKey = BytesToHexString(masterKey)
	s.Data.PasswordHash = BytesToHexString(passwordHash)

	newBlob, err := json.Marshal(s.Data)
	if err != nil {
		return err
	}
	err = s.write(newBlob)
	if err != nil {
		return err
	}

	return nil
}

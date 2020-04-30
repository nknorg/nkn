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

	"github.com/nknorg/nkn/common"
)

type WalletStore struct {
	*WalletData
	Path string
	sync.RWMutex
}

func NewWalletStore(path string, walletData *WalletData) (*WalletStore, error) {
	if common.FileExisted(path) {
		return nil, errors.New("wallet store exists")
	}

	jsonBlob, err := json.Marshal(walletData)
	if err != nil {
		return nil, err
	}

	name := path
	err = ioutil.WriteFile(name, jsonBlob, 0666)
	if err != nil {
		return nil, err
	}

	return &WalletStore{
		Path:       path,
		WalletData: walletData,
	}, nil
}

func LoadWalletStore(fullPath string) (*WalletStore, error) {
	if !common.FileExisted(fullPath) {
		return nil, errors.New("wallet store doesn't exists")
	}

	fileData, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	walletData := &WalletData{}
	err = json.Unmarshal(fileData, walletData)
	if err != nil {
		return nil, err
	}

	return &WalletStore{
		Path:       fullPath,
		WalletData: walletData,
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

func (s *WalletStore) Save() error {
	newBlob, err := json.Marshal(s.WalletData)
	if err != nil {
		return err
	}

	err = s.write(newBlob)
	if err != nil {
		return err
	}

	return nil
}

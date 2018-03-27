package wallet

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	. "nkn-core/common"
	ct "nkn-core/core/contract"
	. "nkn-core/errors"
)

const (
	WalletStoreVersion = "1.0.0"
)

type WalletData struct {
	PasswordHash string
	IV           string
	MasterKey    string
	Version      string
}

type AccountData struct {
	Address             string
	ProgramHash         string
	PrivateKeyEncrypted string
}

type ContractData struct {
	RawData     string
}

type FileData struct {
	WalletData
	Account  AccountData
	Contract ContractData
}

type store struct {
	// this lock could be hold by readDB, writeDB and interrupt signals.
	sync.Mutex

	data FileData
	file *os.File
	path string
}

// Caller holds the lock and reads bytes from DB, then close the DB and release the lock
func (cs *store) readDB() ([]byte, error) {
	cs.Lock()
	defer cs.Unlock()
	defer cs.closeDB()

	var err error
	cs.file, err = os.OpenFile(cs.path, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}

	if cs.file != nil {
		data, err := ioutil.ReadAll(cs.file)
		if err != nil {
			return nil, err
		}
		return data, nil
	} else {
		return nil, NewDetailErr(errors.New("[readDB] file handle is nil"), ErrNoCode, "")
	}
}

// Caller holds the lock and writes bytes to DB, then close the DB and release the lock
func (cs *store) writeDB(data []byte) error {
	cs.Lock()
	defer cs.Unlock()
	defer cs.closeDB()

	var err error
	cs.file, err = os.OpenFile(cs.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}

	if cs.file != nil {
		cs.file.Write(data)
	}

	return nil
}

func (cs *store) closeDB() {
	if cs.file != nil {
		cs.file.Close()
		cs.file = nil
	}
}

func (cs *store) BuildDatabase(path string) {
	os.Remove(path)
	jsonBlob, err := json.Marshal(cs.data)
	if err != nil {
		fmt.Println("Build DataBase Error")
		os.Exit(1)
	}
	cs.writeDB(jsonBlob)
}

func (cs *store) SaveAccountData(programHash []byte, encryptedPrivateKey []byte) error {
	JSONData, err := cs.readDB()
	if err != nil {
		return errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return errors.New("error: unmarshal db")
	}

	pHash, err := Uint160ParseFromBytes(programHash)
	if err != nil {
		return errors.New("invalid program hash")
	}
	addr, err := pHash.ToAddress()
	if err != nil {
		return errors.New("invalid address")
	}
	cs.data.Account = AccountData{
		Address:             addr,
		ProgramHash:         BytesToHexString(programHash),
		PrivateKeyEncrypted: BytesToHexString(encryptedPrivateKey),
	}

	JSONBlob, err := json.Marshal(cs.data)
	if err != nil {
		return errors.New("error: marshal db")
	}
	cs.writeDB(JSONBlob)

	return nil
}

func (cs *store) LoadAccountData() (AccountData, error) {
	JSONData, err := cs.readDB()
	if err != nil {
		return AccountData{}, errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return AccountData{}, errors.New("error: unmarshal db")
	}
	return cs.data.Account, nil
}

func (cs *store) SaveContractData(ct *ct.Contract) error {
	JSONData, err := cs.readDB()
	if err != nil {
		return errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return errors.New("error: unmarshal db")
	}
	cs.data.Contract = ContractData{
		RawData:     BytesToHexString(ct.ToArray()),
	}

	JSONBlob, err := json.Marshal(cs.data)
	if err != nil {
		return errors.New("error: marshal db")
	}
	cs.writeDB(JSONBlob)

	return nil
}

func (cs *store) LoadContractData() (ContractData, error) {
	JSONData, err := cs.readDB()
	if err != nil {
		return ContractData{}, errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return ContractData{}, errors.New("error: unmarshal db")
	}

	return cs.data.Contract, nil
}

func (cs *store) SaveStoredData(name string, value []byte) error {
	JSONData, err := cs.readDB()
	if err != nil {
		return errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return errors.New("error: unmarshal db")
	}

	hexValue := BytesToHexString(value)
	switch name {
	case "Version":
		cs.data.Version = string(value)
	case "IV":
		cs.data.IV = hexValue
	case "MasterKey":
		cs.data.MasterKey = hexValue
	case "PasswordHash":
		cs.data.PasswordHash = hexValue
	}
	JSONBlob, err := json.Marshal(cs.data)
	if err != nil {
		return errors.New("error: marshal db")
	}
	cs.writeDB(JSONBlob)

	return nil
}

func (cs *store) LoadStoredData(name string) ([]byte, error) {
	JSONData, err := cs.readDB()
	if err != nil {
		return nil, errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return nil, errors.New("error: unmarshal db")
	}
	switch name {
	case "Version":
		return []byte(cs.data.Version), nil
	case "IV":
		return HexStringToBytes(cs.data.IV)
	case "MasterKey":
		return HexStringToBytes(cs.data.MasterKey)
	case "PasswordHash":
		return HexStringToBytes(cs.data.PasswordHash)
	}

	return nil, errors.New("Can't find the key: " + name)
}

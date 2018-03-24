package account

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	. "nkn-core/common"
	"nkn-core/common/serialization"
	ct "nkn-core/core/contract"
	"nkn-core/core/transaction"
	. "nkn-core/errors"
)

const (
	WalletStoreVersion = "1.0.0"
)

type WalletData struct {
	PasswordHash string
	IV           string
	MasterKey    string
	Height       uint32
	Version      string
}

type AccountData struct {
	Address             string
	ProgramHash         string
	PrivateKeyEncrypted string
	Type                string
}

type ContractData struct {
	ProgramHash string
	RawData     string
}

type CoinData string

type FileData struct {
	WalletData
	Account  []AccountData
	Contract []ContractData
	Coins    CoinData
}

type FileStore struct {
	// this lock could be hold by readDB, writeDB and interrupt signals.
	sync.Mutex

	data FileData
	file *os.File
	path string
}

// Caller holds the lock and reads bytes from DB, then close the DB and release the lock
func (cs *FileStore) readDB() ([]byte, error) {
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
func (cs *FileStore) writeDB(data []byte) error {
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

func (cs *FileStore) closeDB() {
	if cs.file != nil {
		cs.file.Close()
		cs.file = nil
	}
}

func (cs *FileStore) BuildDatabase(path string) {
	os.Remove(path)
	jsonBlob, err := json.Marshal(cs.data)
	if err != nil {
		fmt.Println("Build DataBase Error")
		os.Exit(1)
	}
	cs.writeDB(jsonBlob)
}

func (cs *FileStore) SaveAccountData(programHash []byte, encryptedPrivateKey []byte) error {
	JSONData, err := cs.readDB()
	if err != nil {
		return errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return errors.New("error: unmarshal db")
	}

	var accountType string
	if len(cs.data.Account) == 0 {
		accountType = MAINACCOUNT
	} else {
		accountType = SUBACCOUNT
	}

	pHash, err := Uint160ParseFromBytes(programHash)
	if err != nil {
		return errors.New("invalid program hash")
	}
	addr, err := pHash.ToAddress()
	if err != nil {
		return errors.New("invalid address")
	}
	a := AccountData{
		Address:             addr,
		ProgramHash:         BytesToHexString(programHash),
		PrivateKeyEncrypted: BytesToHexString(encryptedPrivateKey),
		Type:                accountType,
	}
	cs.data.Account = append(cs.data.Account, a)

	JSONBlob, err := json.Marshal(cs.data)
	if err != nil {
		return errors.New("error: marshal db")
	}
	cs.writeDB(JSONBlob)

	return nil
}

func (cs *FileStore) DeleteAccountData(programHash string) error {
	JSONData, err := cs.readDB()
	if err != nil {
		return errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return errors.New("error: unmarshal db")
	}

	for i, v := range cs.data.Account {
		if programHash == v.ProgramHash {
			if v.Type == MAINACCOUNT {
				return errors.New("Can't remove main account")
			}
			cs.data.Account = append(cs.data.Account[:i], cs.data.Account[i+1:]...)
		}
	}

	JSONBlob, err := json.Marshal(cs.data)
	if err != nil {
		return errors.New("error: marshal db")
	}
	cs.writeDB(JSONBlob)

	return nil
}

func (cs *FileStore) LoadAccountData() ([]AccountData, error) {
	JSONData, err := cs.readDB()
	if err != nil {
		return nil, errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return nil, errors.New("error: unmarshal db")
	}
	return cs.data.Account, nil
}

func (cs *FileStore) SaveContractData(ct *ct.Contract) error {
	JSONData, err := cs.readDB()
	if err != nil {
		return errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return errors.New("error: unmarshal db")
	}
	c := ContractData{
		ProgramHash: BytesToHexString(ct.ProgramHash.ToArray()),
		RawData:     BytesToHexString(ct.ToArray()),
	}
	cs.data.Contract = append(cs.data.Contract, c)

	JSONBlob, err := json.Marshal(cs.data)
	if err != nil {
		return errors.New("error: marshal db")
	}
	cs.writeDB(JSONBlob)

	return nil
}

func (cs *FileStore) DeleteContractData(programHash string) error {
	JSONData, err := cs.readDB()
	if err != nil {
		return errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return errors.New("error: unmarshal db")
	}

	for i, v := range cs.data.Contract {
		if programHash == v.ProgramHash {
			cs.data.Contract = append(cs.data.Contract[:i], cs.data.Contract[i+1:]...)
		}
	}

	JSONBlob, err := json.Marshal(cs.data)
	if err != nil {
		return errors.New("error: marshal db")
	}
	cs.writeDB(JSONBlob)

	return nil
}

func (cs *FileStore) LoadContractData() ([]ContractData, error) {
	JSONData, err := cs.readDB()
	if err != nil {
		return nil, errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return nil, errors.New("error: unmarshal db")
	}

	return cs.data.Contract, nil
}

func (cs *FileStore) SaveCoinsData(coins map[*transaction.UTXOTxInput]*Coin) error {
	JSONData, err := cs.readDB()
	if err != nil {
		return errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return errors.New("error: unmarshal db")
	}

	length := uint32(len(coins))
	if length == 0 {
		cs.data.Coins = ""
	} else {
		w := new(bytes.Buffer)
		serialization.WriteUint32(w, uint32(len(coins)))
		for k, v := range coins {
			k.Serialize(w)
			v.Serialize(w, cs.data.Version)
		}
		cs.data.Coins = CoinData(BytesToHexString(w.Bytes()))
	}

	JSONBlob, err := json.Marshal(cs.data)
	if err != nil {
		return errors.New("error: marshal db")
	}
	cs.writeDB(JSONBlob)

	return nil
}

func (cs *FileStore) DeleteCoinsData(programHash Uint160) error {
	JSONData, err := cs.readDB()
	if err != nil {
		return errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return errors.New("error: unmarshal db")
	}
	if cs.data.Coins == "" {
		return nil
	}

	coins := make(map[*transaction.UTXOTxInput]*Coin)
	rawCoins, _ := HexStringToBytes(string(cs.data.Coins))
	r := bytes.NewReader(rawCoins)
	num, _ := serialization.ReadUint32(r)
	for i := 0; i < int(num); i++ {
		input := new(transaction.UTXOTxInput)
		if err := input.Deserialize(r); err != nil {
			return err
		}
		coin := new(Coin)
		if err := coin.Deserialize(r, cs.data.Version); err != nil {
			return err
		}
		if coin.Output.ProgramHash != programHash {
			coins[input] = coin
		}
	}
	if err := cs.SaveCoinsData(coins); err != nil {
		return err
	}

	return nil
}

func (cs *FileStore) LoadCoinsData() (map[*transaction.UTXOTxInput]*Coin, error) {
	JSONData, err := cs.readDB()
	if err != nil {
		return nil, errors.New("error: reading db")
	}
	if err := json.Unmarshal(JSONData, &cs.data); err != nil {
		return nil, errors.New("error: unmarshal db")
	}
	coins := make(map[*transaction.UTXOTxInput]*Coin)
	rawCoins, _ := HexStringToBytes(string(cs.data.Coins))
	r := bytes.NewReader(rawCoins)
	num, _ := serialization.ReadUint32(r)
	for i := 0; i < int(num); i++ {
		input := new(transaction.UTXOTxInput)
		if err := input.Deserialize(r); err != nil {
			return nil, err
		}
		coin := new(Coin)
		if err := coin.Deserialize(r, cs.data.Version); err != nil {
			return nil, err
		}
		coins[input] = coin
	}

	return coins, nil
}

func (cs *FileStore) SaveStoredData(name string, value []byte) error {
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
	case "Height":
		var height uint32
		bytesBuffer := bytes.NewBuffer(value)
		binary.Read(bytesBuffer, binary.LittleEndian, &height)
		cs.data.Height = height

	}
	JSONBlob, err := json.Marshal(cs.data)
	if err != nil {
		return errors.New("error: marshal db")
	}
	cs.writeDB(JSONBlob)

	return nil
}

func (cs *FileStore) LoadStoredData(name string) ([]byte, error) {
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
	case "Height":
		bytesBuffer := bytes.NewBuffer([]byte{})
		binary.Write(bytesBuffer, binary.LittleEndian, cs.data.Height)
		return bytesBuffer.Bytes(), nil
	}

	return nil, errors.New("Can't find the key: " + name)
}

package ledger

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/asset"
	"github.com/nknorg/nkn/core/contract/program"
	sig "github.com/nknorg/nkn/core/signature"
	tx "github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/util/log"
)

const BlockVersion uint32 = 0
const GenesisNonce uint64 = 2083236893

type Block struct {
	Header       *Header
	Transactions []*tx.Transaction

	hash *Uint256
}

func (b *Block) Serialize(w io.Writer) error {
	b.Header.Serialize(w)
	err := serialization.WriteUint32(w, uint32(len(b.Transactions)))
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Block item Transactions length serialization failed.")
	}

	for _, transaction := range b.Transactions {
		transaction.Serialize(w)
	}
	return nil
}

func (b *Block) Deserialize(r io.Reader) error {
	if b.Header == nil {
		b.Header = new(Header)
	}
	b.Header.Deserialize(r)

	//Transactions
	var i uint32
	Len, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	var txhash Uint256
	var tharray []Uint256
	for i = 0; i < Len; i++ {
		transaction := new(tx.Transaction)
		transaction.Deserialize(r)
		txhash = transaction.Hash()
		b.Transactions = append(b.Transactions, transaction)
		tharray = append(tharray, txhash)
	}

	b.Header.TransactionsRoot, err = crypto.ComputeRoot(tharray)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Block Deserialize merkleTree compute failed")
	}

	return nil
}

func (b *Block) GetSigner() ([]byte, error) {
	return b.Header.Signer, nil
}

func (b *Block) Trim(w io.Writer) error {
	b.Header.Serialize(w)
	err := serialization.WriteUint32(w, uint32(len(b.Transactions)))
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Block item Transactions length serialization failed.")
	}
	for _, transaction := range b.Transactions {
		temp := *transaction
		hash := temp.Hash()
		hash.Serialize(w)
	}
	return nil
}

func (b *Block) FromTrimmedData(r io.Reader) error {
	if b.Header == nil {
		b.Header = new(Header)
	}
	b.Header.Deserialize(r)

	//Transactions
	var i uint32
	Len, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	var txhash Uint256
	var tharray []Uint256
	for i = 0; i < Len; i++ {
		txhash.Deserialize(r)
		transaction := new(tx.Transaction)
		transaction.SetHash(txhash)
		b.Transactions = append(b.Transactions, transaction)
		tharray = append(tharray, txhash)
	}

	b.Header.TransactionsRoot, err = crypto.ComputeRoot(tharray)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Block Deserialize merkleTree compute failed")
	}

	return nil
}

func (b *Block) GetMessage() []byte {
	return sig.GetHashData(b)
}

func (b *Block) ToArray() []byte {
	bf := new(bytes.Buffer)
	b.Serialize(bf)
	return bf.Bytes()
}

func (b *Block) GetProgramHashes() ([]Uint160, error) {

	return b.Header.GetProgramHashes()
}

func (b *Block) SetPrograms(prog []*program.Program) {
	b.Header.SetPrograms(prog)
	return
}

func (b *Block) GetPrograms() []*program.Program {
	return b.Header.GetPrograms()
}

func (b *Block) Hash() Uint256 {
	if b.hash == nil {
		b.hash = new(Uint256)
		*b.hash = b.Header.Hash()
	}
	return *b.hash
}

func (b *Block) Verify() error {
	log.Info("This function is expired.please use Validation/blockValidator to Verify Block.")
	return nil
}

func (b *Block) Type() InventoryType {
	return BLOCK
}

func GenesisBlockInit() (*Block, error) {
	// block header
	genesisBlockHeader := &Header{
		Version:          BlockVersion,
		PrevBlockHash:    Uint256{},
		TransactionsRoot: Uint256{},
		Timestamp:        time.Date(2018, time.January, 0, 0, 0, 0, 0, time.UTC).Unix(),
		Height:           uint32(0),
		ConsensusData:    GenesisNonce,
		NextBookKeeper:   Uint160{},
		Program: &program.Program{
			Code:      []byte{0x00},
			Parameter: []byte{0x00},
		},
	}
	// asset transaction
	trans := &tx.Transaction{
		TxType:         tx.RegisterAsset,
		PayloadVersion: 0,
		Payload: &payload.RegisterAsset{
			Asset: &asset.Asset{
				Name:        "NKN",
				Description: "NKN Test Token",
				Precision:   MaximumPrecision,
			},
			Amount: 700000000 * StorageFactor,
			Issuer: &crypto.PubKey{
				X: big.NewInt(0),
				Y: big.NewInt(0),
			},
			Controller: Uint160{},
		},
		Attributes: []*tx.TxnAttribute{},
		Programs: []*program.Program{
			{
				Code:      []byte{0x00},
				Parameter: []byte{0x00},
			},
		},
	}
	// genesis block
	genesisBlock := &Block{
		Header:       genesisBlockHeader,
		Transactions: []*tx.Transaction{trans},
	}

	return genesisBlock, nil
}

func (b *Block) RebuildMerkleRoot() error {
	txs := b.Transactions
	transactionHashes := []Uint256{}
	for _, tx := range txs {
		transactionHashes = append(transactionHashes, tx.Hash())
	}
	hash, err := crypto.ComputeRoot(transactionHashes)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "[Block] , RebuildMerkleRoot ComputeRoot failed.")
	}
	b.Header.TransactionsRoot = hash
	return nil

}

func (bd *Block) SerializeUnsigned(w io.Writer) error {
	return bd.Header.SerializeUnsigned(w)
}

func (bd *Block) MarshalJson() ([]byte, error) {
	var blockInfo BlocksInfo

	info, err := bd.Header.MarshalJson()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(info, &blockInfo.Header)
	if err != nil {
		return nil, err
	}

	for _, v := range bd.Transactions {
		info, err := v.MarshalJson()
		if err != nil {
			return nil, err
		}
		var t tx.TransactionInfo
		err = json.Unmarshal(info, &t)
		if err != nil {
			return nil, err
		}
		blockInfo.Transactions = append(blockInfo.Transactions, &t)
	}

	if bd.hash != nil {
		blockInfo.Hash = BytesToHexString(bd.hash.ToArrayReverse())
	}

	data, err := json.Marshal(blockInfo)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (bd *Block) UnmarshalJson(data []byte) error {
	blockInfo := new(BlocksInfo)
	var err error
	if err = json.Unmarshal(data, &blockInfo); err != nil {
		return err
	}

	info, err := json.Marshal(blockInfo.Header)
	if err != nil {
		return err
	}
	var header Header
	err = header.UnmarshalJson(info)
	bd.Header = &header

	for _, v := range blockInfo.Transactions {
		info, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var txn tx.Transaction
		err = txn.UnmarshalJson(info)
		if err != nil {
			return err
		}
		bd.Transactions = append(bd.Transactions, &txn)
	}

	if blockInfo.Hash != "" {
		hashSlice, err := HexStringToBytesReverse(blockInfo.Hash)
		if err != nil {
			return err
		}
		hash, err := Uint256ParseFromBytes(hashSlice)
		if err != nil {
			return err
		}
		bd.hash = &hash
	}

	return nil
}

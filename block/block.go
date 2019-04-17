package block

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/pb"
	. "github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/vm/signature"
)

const BlockVersion uint32 = 0
const GenesisNonce uint64 = 2083236893

type Block struct {
	Header       *Header
	Transactions []*Transaction

	hash *Uint256
}

func (b *Block) ToMsgBlock() *MsgBlock {
	msgBlock := &MsgBlock{
		Header: &b.Header.BlockHeader,
	}

	for _, txn := range b.Transactions {
		msgBlock.Transactions = append(msgBlock.Transactions, &txn.MsgTx)
	}

	return msgBlock
}

func (b *Block) Marshal() (dAtA []byte, err error) {
	msgBlock := &MsgBlock{
		Header: &b.Header.BlockHeader,
	}

	for _, txn := range b.Transactions {
		msgBlock.Transactions = append(msgBlock.Transactions, &txn.MsgTx)
	}

	return msgBlock.Marshal()
}

func (b *Block) Unmarshal(dAtA []byte) error {
	var msgBlock MsgBlock
	msgBlock.Unmarshal(dAtA)
	b.Header = &Header{BlockHeader: *msgBlock.Header}
	for _, txn := range msgBlock.Transactions {
		b.Transactions = append(b.Transactions, &Transaction{MsgTx: *txn})
	}

	return nil
}

func (b *Block) Serialize(w io.Writer) error {
	return nil
}

func (b *Block) Deserialize(r io.Reader) error {
	return nil
}

func (b *Block) GetTxsSize() int {
	txnSize := 0
	for _, txn := range b.Transactions {
		txnSize += txn.GetSize()
	}

	return txnSize
}

func (b *Block) GetSigner() ([]byte, []byte, error) {
	return b.Header.UnsignedHeader.Signer, b.Header.UnsignedHeader.ChordId, nil
}

func (b *Block) Trim(w io.Writer) error {
	dt, _ := b.Header.Marshal()
	serialization.WriteVarBytes(w, dt)
	err := serialization.WriteUint32(w, uint32(len(b.Transactions)))
	if err != nil {
		return fmt.Errorf("Block item Transactions length serialization failed: %v", err)
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

	dt, _ := serialization.ReadVarBytes(r)
	b.Header.Unmarshal(dt)

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
		transaction := new(Transaction)
		transaction.SetHash(txhash)
		b.Transactions = append(b.Transactions, transaction)
		tharray = append(tharray, txhash)
	}

	root, err := crypto.ComputeRoot(tharray)
	if err != nil {
		return fmt.Errorf("Block Deserialize merkleTree compute failed: %v", err)
	}
	b.Header.UnsignedHeader.TransactionsRoot = root.ToArray()

	return nil
}

func (b *Block) GetMessage() []byte {
	return signature.GetHashData(b)
}

func (b *Block) ToArray() []byte {
	dt, _ := b.Marshal()
	return dt
}

func (b *Block) GetProgramHashes() ([]Uint160, error) {

	return b.Header.GetProgramHashes()
}

func (b *Block) SetPrograms(prog []*Program) {
	b.Header.SetPrograms(prog)
	return
}

func (b *Block) GetPrograms() []*Program {
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
	return nil
}

func (b *Block) Type() InventoryType {
	return BLOCK
}

func GenesisBlockInit() (*Block, error) {
	if config.Parameters.GenesisBlockProposer == "" {
		return nil, errors.New("GenesisBlockProposer is required in config.json")
	}
	proposer, err := HexStringToBytes(config.Parameters.GenesisBlockProposer)
	if err != nil || len(proposer) != crypto.COMPRESSEDLEN {
		return nil, errors.New("invalid GenesisBlockProposer configured")
	}
	genesisBlockProposer, _ := HexStringToBytes(config.Parameters.GenesisBlockProposer)
	// block header
	genesisBlockHeader := &Header{
		BlockHeader: BlockHeader{
			UnsignedHeader: &UnsignedHeader{
				Version:       BlockVersion,
				PrevBlockHash: EmptyUint256.ToArray(),
				Timestamp:     time.Date(2018, time.January, 0, 0, 0, 0, 0, time.UTC).Unix(),

				Height:        uint32(0),
				ConsensusData: GenesisNonce,
				Signer:        genesisBlockProposer,
			},
		},
	}

	rewardAddress, _ := ToScriptHash(config.InitialIssueAddress)
	payload := NewCoinbase(EmptyUint160, rewardAddress, Fixed64(config.InitialIssueAmount))
	pl, err := Pack(CoinbaseType, payload)
	if err != nil {
		return nil, err
	}

	txn := NewMsgTx(pl, 0, 0, []byte{})
	txn.Programs = []*Program{
		{
			Code:      []byte{0x00},
			Parameter: []byte{0x00},
		},
	}
	trans := &Transaction{
		MsgTx: *txn,
	}

	// genesis block
	genesisBlock := &Block{
		Header:       genesisBlockHeader,
		Transactions: []*Transaction{trans},
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
		return fmt.Errorf("[Block] , RebuildMerkleRoot ComputeRoot failed: %v", err)
	}
	b.Header.UnsignedHeader.TransactionsRoot = hash.ToArray()
	return nil

}

func (b *Block) SerializeUnsigned(w io.Writer) error {
	return b.Header.SerializeUnsigned(w)
}

func (b *Block) GetInfo() ([]byte, error) {
	type blockInfo struct {
		Header       interface{}   `json:"header"`
		Transactions []interface{} `json:"transactions"`
		Size         int           `json:"size"`
		Hash         string        `json:"hash"`
	}

	var unmarshaledHeader interface{}
	headerInfo, _ := b.Header.GetInfo()
	json.Unmarshal(headerInfo, &unmarshaledHeader)

	b.Hash()
	info := &blockInfo{
		Header:       unmarshaledHeader,
		Transactions: make([]interface{}, 0),
		Size:         b.ToMsgBlock().Size(),
		Hash:         b.hash.ToHexString(),
	}

	for _, v := range b.Transactions {
		var unmarshaledTxn interface{}
		txnInfo, _ := v.GetInfo()
		json.Unmarshal(txnInfo, &unmarshaledTxn)
		info.Transactions = append(info.Transactions, unmarshaledTxn)
	}

	marshaledInfo, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}

	return marshaledInfo, nil
}

package block

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/gogo/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/signature"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
)

type Block struct {
	Header       *Header
	Transactions []*transaction.Transaction
}

func (b *Block) FromMsgBlock(msgBlock *pb.Block) {
	b.Header = &Header{Header: msgBlock.Header}
	for _, txn := range msgBlock.Transactions {
		b.Transactions = append(b.Transactions, &transaction.Transaction{Transaction: txn})
	}
}

func (b *Block) ToMsgBlock() *pb.Block {
	if b == nil {
		return nil
	}

	msgBlock := &pb.Block{
		Header: b.Header.Header,
	}

	for _, txn := range b.Transactions {
		msgBlock.Transactions = append(msgBlock.Transactions, txn.Transaction)
	}

	return msgBlock
}

func (b *Block) Marshal() (data []byte, err error) {
	return proto.Marshal(b.ToMsgBlock())
}

func (b *Block) Unmarshal(buf []byte) error {
	msgBlock := &pb.Block{}
	err := proto.Unmarshal(buf, msgBlock)
	if err != nil {
		return err
	}
	b.FromMsgBlock(msgBlock)
	return nil
}

func (b *Block) GetTxsSize() int {
	txnSize := 0
	for _, txn := range b.Transactions {
		txnSize += int(txn.GetSize())
	}

	return txnSize
}

func (b *Block) GetSigner() ([]byte, []byte, error) {
	return b.Header.UnsignedHeader.SignerPk, b.Header.UnsignedHeader.SignerId, nil
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
		transaction := new(transaction.Transaction)
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

func (b *Block) SetPrograms(prog []*pb.Program) {
	b.Header.SetPrograms(prog)
	return
}

func (b *Block) GetPrograms() []*pb.Program {
	return b.Header.GetPrograms()
}

func (b *Block) Hash() Uint256 {
	return b.Header.Hash()
}

func (b *Block) Verify() error {
	return nil
}

func ComputeID(preBlockHash, txnHash Uint256, randomBeacon []byte) []byte {
	data := append(preBlockHash[:], txnHash[:]...)
	data = append(data, randomBeacon...)
	id := crypto.Sha256(data)
	return id
}

func GenesisBlockInit() (*Block, error) {
	genesisSignerPk, err := HexStringToBytes(config.Parameters.GenesisBlockProposer)
	if err != nil {
		return nil, fmt.Errorf("parse GenesisBlockProposer error: %v", err)
	}

	genesisSignerID := ComputeID(EmptyUint256, EmptyUint256, config.GenesisBeacon[:config.RandomBeaconUniqueLength])

	// block header
	genesisBlockHeader := &Header{
		Header: &pb.Header{
			UnsignedHeader: &pb.UnsignedHeader{
				Version:       config.HeaderVersion,
				PrevBlockHash: EmptyUint256.ToArray(),
				Timestamp:     config.GenesisTimestamp,
				Height:        uint32(0),
				RandomBeacon:  config.GenesisBeacon,
				SignerPk:      genesisSignerPk,
				SignerId:      genesisSignerID,
			},
		},
	}

	rewardAddress, err := ToScriptHash(config.InitialIssueAddress)
	if err != nil {
		return nil, fmt.Errorf("parse InitialIssueAddress error: %v", err)
	}
	donationProgramhash, err := ToScriptHash(config.DonationAddress)
	if err != nil {
		return nil, fmt.Errorf("parse DonationAddress error: %v", err)
	}
	payload := transaction.NewCoinbase(donationProgramhash, rewardAddress, Fixed64(0))
	pl, err := transaction.Pack(pb.COINBASE_TYPE, payload)
	if err != nil {
		return nil, err
	}

	txn := transaction.NewMsgTx(pl, 0, 0, []byte{})
	txn.Programs = []*pb.Program{
		{
			Code:      []byte{0x00},
			Parameter: []byte{0x00},
		},
	}
	trans := &transaction.Transaction{
		Transaction: txn,
	}

	// genesis block
	genesisBlock := &Block{
		Header:       genesisBlockHeader,
		Transactions: []*transaction.Transaction{trans},
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

	hash := b.Hash()
	info := &blockInfo{
		Header:       unmarshaledHeader,
		Transactions: make([]interface{}, 0),
		Size:         b.ToMsgBlock().Size(),
		Hash:         hash.ToHexString(),
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

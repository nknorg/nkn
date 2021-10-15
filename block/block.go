package block

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/common/serialization"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/signature"
	"github.com/nknorg/nkn/v2/transaction"
)

type Block struct {
	Header        *Header
	Transactions  []*transaction.Transaction
	IsTxnsChecked bool
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

func (b *Block) Trim(w io.Writer) error {
	dt, err := b.Header.Marshal()
	if err != nil {
		return err
	}
	err = serialization.WriteVarBytes(w, dt)
	if err != nil {
		return err
	}
	err = serialization.WriteUint32(w, uint32(len(b.Transactions)))
	if err != nil {
		return err
	}
	for _, tx := range b.Transactions {
		hash := tx.Hash()
		_, err = hash.Serialize(w)
		if err != nil {
			return err
		}
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
	var txhash common.Uint256
	var tharray []common.Uint256
	for i = 0; i < Len; i++ {
		err = txhash.Deserialize(r)
		if err != nil {
			return err
		}
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

func (b *Block) GetProgramHashes() ([]common.Uint160, error) {

	return b.Header.GetProgramHashes()
}

func (b *Block) SetPrograms(prog []*pb.Program) {
	b.Header.SetPrograms(prog)
	return
}

func (b *Block) GetPrograms() []*pb.Program {
	return b.Header.GetPrograms()
}

func (b *Block) Hash() common.Uint256 {
	return b.Header.Hash()
}

func (b *Block) Verify() error {
	return nil
}

func ComputeID(preBlockHash, txnHash common.Uint256, randomBeacon []byte) []byte {
	data := append(preBlockHash[:], txnHash[:]...)
	data = append(data, randomBeacon...)
	id := sha256.Sum256(data)
	return id[:]
}

func GenesisBlockInit() (*Block, error) {
	genesisSignerPk, err := hex.DecodeString(config.Parameters.GenesisBlockProposer)
	if err != nil {
		return nil, fmt.Errorf("parse GenesisBlockProposer error: %v", err)
	}

	genesisSignerID := ComputeID(common.EmptyUint256, common.EmptyUint256, config.GenesisBeacon[:config.RandomBeaconUniqueLength])

	// block header
	genesisBlockHeader := &Header{
		Header: &pb.Header{
			UnsignedHeader: &pb.UnsignedHeader{
				Version:       config.HeaderVersion,
				PrevBlockHash: common.EmptyUint256.ToArray(),
				Timestamp:     config.GenesisTimestamp,
				Height:        uint32(0),
				RandomBeacon:  config.GenesisBeacon,
				SignerPk:      genesisSignerPk,
				SignerId:      genesisSignerID,
			},
		},
	}

	rewardAddress, err := common.ToScriptHash(config.InitialIssueAddress)
	if err != nil {
		return nil, fmt.Errorf("parse InitialIssueAddress error: %v", err)
	}
	donationProgramhash, err := common.ToScriptHash(config.DonationAddress)
	if err != nil {
		return nil, fmt.Errorf("parse DonationAddress error: %v", err)
	}
	payload := transaction.NewCoinbase(donationProgramhash, rewardAddress, common.Fixed64(0))
	pl, err := transaction.Pack(pb.PayloadType_COINBASE_TYPE, payload)
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
	transactionHashes := []common.Uint256{}
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
		Size:         proto.Size(b.ToMsgBlock()),
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

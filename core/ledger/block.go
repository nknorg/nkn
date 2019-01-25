package ledger

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/contract/program"
	sig "github.com/nknorg/nkn/core/signature"
	tx "github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/types"
	"github.com/nknorg/nkn/util/config"
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

	root, err := crypto.ComputeRoot(tharray)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Block Deserialize merkleTree compute failed")
	}
	b.Header.UnsignedHeader.TransactionsRoot = root.ToArray()

	return nil
}

func (b *Block) GetSigner() ([]byte, []byte, error) {
	return b.Header.UnsignedHeader.Signer, b.Header.UnsignedHeader.ChordID, nil
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

	root, err := crypto.ComputeRoot(tharray)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "Block Deserialize merkleTree compute failed")
	}
	b.Header.UnsignedHeader.TransactionsRoot = root.ToArray()

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
		BlockHeader: types.BlockHeader{
			UnsignedHeader: &types.UnsignedHeader{
				Version:       BlockVersion,
				PrevBlockHash: EmptyUint256.ToArray(),
				Timestamp:     time.Date(2018, time.January, 0, 0, 0, 0, 0, time.UTC).Unix(),

				Height:         uint32(0),
				ConsensusData:  GenesisNonce,
				NextBookKeeper: EmptyUint160.ToArray(),
				Signer:         genesisBlockProposer,
			},
			Program: &types.Program{
				Code:      []byte{0x00},
				Parameter: []byte{0x00},
			},
		},
	}

	rewardAddress, _ := ToScriptHash("NcX9BWx5uxsevCZ2MUEbBJGoYGSNCuJJpf")
	payload := types.NewCoinbase(EmptyUint160, rewardAddress, Fixed64(config.DefaultMiningReward*StorageFactor))
	pl, err := types.Pack(types.CoinbaseType, payload)
	if err != nil {
		return nil, err
	}

	txn := types.NewMsgTx(pl, 0, 0, []byte{})
	txn.Programs = []*types.Program{
		{
			Code:      []byte{0x00},
			Parameter: []byte{0x00},
		},
	}
	trans := &tx.Transaction{
		MsgTx: *txn,
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
	b.Header.UnsignedHeader.TransactionsRoot = hash.ToArray()
	return nil

}

func (bd *Block) SerializeUnsigned(w io.Writer) error {
	return bd.Header.SerializeUnsigned(w)
}

func (bd *Block) MarshalJson() ([]byte, error) {
	return json.Marshal(bd)
}

func (bd *Block) UnmarshalJson(data []byte) error {
	return json.Unmarshal(data, bd)
}

package por

import (
	"bytes"
	"errors"

	"github.com/gogo/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
)

const (
	// Block proposer of block height X needs to be specified in block X-1, so its
	// candidate sigchains have to be fully propogated before block X-1 is
	// proposed, i.e. before block X-2 is accepted. In other words, sigchain can
	// only propogate when block height <= X-3.
	SigChainPropagationHeightOffset = 3
	// Block proposer of block height X is chosen from sigchain produced during
	// block X-SigChainPropagationHeightOffset-SigChainPropogationTime because
	// sigchain can only propogate when block height <=
	// X-SigChainPropagationHeightOffset, so it has to start propogating when
	// block height <= X-SigChainPropagationHeightOffset-SigChainPropogationTime.
	SigChainMiningHeightOffset = config.SigChainPropogationTime + SigChainPropagationHeightOffset
)

type PorPackage struct {
	VoteForHeight uint32       `protobuf:"varint,1,opt,name=VoteForHeight,proto3" json:"VoteForHeight,omitempty"`
	BlockHash     []byte       `protobuf:"bytes,3,opt,name=BlockHash,proto3" json:"BlockHash,omitempty"`
	TxHash        []byte       `protobuf:"bytes,4,opt,name=TxHash,proto3" json:"TxHash,omitempty"`
	SigHash       []byte       `protobuf:"bytes,5,opt,name=SigHash,proto3" json:"SigHash,omitempty"`
	SigChain      *pb.SigChain `protobuf:"bytes,6,opt,name=SigChain" json:"SigChain,omitempty"`
}

type PorPackages []*PorPackage

func (c PorPackages) Len() int {
	return len(c)
}

func (c PorPackages) Swap(i, j int) {
	if i >= 0 && i < len(c) && j >= 0 && j < len(c) { // Unit Test modify
		c[i], c[j] = c[j], c[i]
	}
}

func (c PorPackages) Less(i, j int) bool {
	if i >= 0 && i < len(c) && j >= 0 && j < len(c) { // Unit Test modify
		return bytes.Compare(c[i].SigHash, c[j].SigHash) < 0
	}

	return false
}

func NewPorPackage(txn *transaction.Transaction, shouldVerify bool) (*PorPackage, error) {
	if txn.UnsignedTx.Payload.Type != pb.SIG_CHAIN_TXN_TYPE {
		return nil, errors.New("Transaction type mismatch")
	}
	payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return nil, err
	}

	rs := payload.(*pb.SigChainTxn)
	sigChain := &pb.SigChain{}
	err = proto.Unmarshal(rs.SigChain, sigChain)
	if err != nil {
		return nil, err
	}

	found := false
	for _, elem := range sigChain.Elems {
		if elem.Mining == true {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("No miner node in signature chain")
	}

	blockHash, err := Uint256ParseFromBytes(sigChain.BlockHash)
	if err != nil {
		return nil, err
	}

	if blockHash == EmptyUint256 {
		return nil, errors.New("block hash in sigchain is empty")
	}

	height, err := Store.GetHeightByBlockHash(blockHash)
	if err != nil {
		return nil, err
	}

	txHash := txn.Hash()
	sigHash, err := sigChain.SignatureHash()
	if err != nil {
		return nil, err
	}

	if shouldVerify {
		err = VerifyID(sigChain)
		if err != nil {
			return nil, err
		}

		err = sigChain.Verify()
		if err != nil {
			return nil, err
		}
	}

	pp := &PorPackage{
		VoteForHeight: height + SigChainMiningHeightOffset + config.SigChainBlockDelay,
		BlockHash:     sigChain.BlockHash,
		TxHash:        txHash.ToArray(),
		SigHash:       sigHash,
		SigChain:      sigChain,
	}
	return pp, nil
}

package ising

import (
	"sync"

	"github.com/gogo/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/consensus/ising/voting"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

const (
	InitialBlockHeight = 5
)

type ProposerInfo struct {
	publicKey       []byte
	chordID         []byte
	winnerHash      Uint256
	winnerType      ledger.WinnerType
}

type ProposerCache struct {
	sync.RWMutex
	cache map[uint32]*ProposerInfo
}

func NewProposerCache() *ProposerCache {
	return &ProposerCache{
		cache: make(map[uint32]*ProposerInfo),
	}
}

func (pc *ProposerCache) Add(height uint32, votingContent voting.VotingContent) {
	pc.Lock()
	defer pc.Unlock()

	var proposerInfo *ProposerInfo
	var pbk, id []byte
	var err error
	switch t := votingContent.(type) {
	case *ledger.Block:
		pbk, id, _ = t.GetSigner()
		proposerInfo = &ProposerInfo{
			publicKey:       pbk,
			chordID:         id,
			winnerHash:      t.Hash(),
			winnerType:      ledger.BlockSigner,
		}
		log.Warningf("use proposer of block height %d which public key is %s chord ID is %s to propose block %d",
			t.Header.Height, BytesToHexString(pbk), BytesToHexString(id), height)
	case *transaction.Transaction:
		payload := t.Payload.(*payload.Commit)
		sigchain := &por.SigChain{}
		proto.Unmarshal(payload.SigChain, sigchain)
		pbk, id, err = sigchain.GetMiner()
		if err != nil {
			log.Warning("Get last public key error", err)
			return
		}
		proposerInfo = &ProposerInfo{
			publicKey:      pbk,
			chordID:        id,
			winnerHash:	t.Hash(),
			winnerType:	ledger.TxnSigner,
		}
		sigChainTxnHash := t.Hash()
		log.Infof("sigchain transaction consensus: %s, %s will be block proposer for height %d",
			BytesToHexString(sigChainTxnHash.ToArrayReverse()), BytesToHexString(pbk), height)
	}

	pc.cache[height] = proposerInfo
}

func (pc *ProposerCache) Get(height uint32) *ProposerInfo {
	pc.RLock()
	defer pc.RUnlock()

	// initial blocks are produced by GenesisBlockProposer
	if height < InitialBlockHeight {
		proposer, _ := HexStringToBytes(config.Parameters.GenesisBlockProposer)
		return &ProposerInfo{
			publicKey:      proposer,
			winnerHash:     EmptyUint256,
			winnerType:	ledger.GenesisSigner,
		}
	}

	if _, ok := pc.cache[height]; ok {
		return pc.cache[height]
	}

	return nil
}

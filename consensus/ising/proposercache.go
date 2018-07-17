package ising

import (
	"errors"
	"fmt"
	"sync"

	"github.com/golang/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/consensus/ising/voting"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

const (
	InitialBlockHeight = 5
)

type ProposerInfo struct {
	publicKey       []byte
	winningHash     Uint256
	winningHashType ledger.WinningHashType
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
	switch t := votingContent.(type) {
	case *ledger.Block:
		signer, _ := t.GetSigner()
		proposerInfo = &ProposerInfo{
			publicKey:       signer,
			winningHash:     t.Hash(),
			winningHashType: ledger.WinningBlockHash,
		}
		log.Warnf("use proposer of block height %d which public key is %s to propose block %d",
			t.Header.Height, BytesToHexString(signer), height)
	case *transaction.Transaction:
		payload := t.Payload.(*payload.Commit)
		sigchain := &por.SigChain{}
		proto.Unmarshal(payload.SigChain, sigchain)
		// TODO: get a determinate public key on signature chain
		pbk, err := sigchain.GetMiner()
		if err != nil {
			log.Warn("Get last public key error", err)
			return
		}
		proposerInfo = &ProposerInfo{
			publicKey:       pbk,
			winningHash:     t.Hash(),
			winningHashType: ledger.WinningTxnHash,
		}
		sigChainTxnHash := t.Hash()
		log.Infof("sigchain transaction consensus: %s, %s will be block proposer for height %d",
			BytesToHexString(sigChainTxnHash.ToArrayReverse()), BytesToHexString(pbk), height)
	}

	pc.cache[height] = proposerInfo
}

func (pc *ProposerCache) Get(height uint32) (*ProposerInfo, error) {
	pc.RLock()
	defer pc.RUnlock()
	// initial blocks are produced byte GenesisBlockProposer
	if height < InitialBlockHeight {
		if len(config.Parameters.GenesisBlockProposer) < 1 {
			err := errors.New("no GenesisBlockProposer configured")
			log.Warn(err)
			return nil, err
		}
		proposer, err := HexStringToBytes(config.Parameters.GenesisBlockProposer[0])
		if err != nil || len(proposer) != crypto.COMPRESSEDLEN {
			err := errors.New("invalid GenesisBlockProposer configured")
			return nil, err
		}

		return &ProposerInfo{
			publicKey:       proposer,
			winningHash:     EmptyUint256,
			winningHashType: ledger.GenesisHash,
		}, nil
	}

	if _, ok := pc.cache[height]; !ok {
		return nil, fmt.Errorf("no proposer info for height: ", height)
	}

	return pc.cache[height], nil
}

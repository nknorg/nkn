package ledger

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/core/signature"
	tx "github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

const (
	GenesisBlockProposedHeight = 4
)

func BlockSanityCheck(block *Block, ledger *Ledger) error {
	// TODO: verify block signature
	if block.Transactions == nil {
		return errors.New("empty block")
	}
	if block.Transactions[0].TxType != tx.Coinbase {
		return errors.New("first transaction in block is not Coinbase")
	}
	for _, txn := range block.Transactions[1:] {
		if txn.TxType == tx.Coinbase {
			return errors.New("Coinbase transaction order is incorrect")
		}
	}

	return nil
}

func BlockFullyCheck(block *Block, ledger *Ledger) error {
	if err := BlockSanityCheck(block, ledger); err != nil {
		return errors.New("block sanity check error")
	}
	err := HeaderCheck(block.Header, ledger)
	if err != nil {
		return err
	}
	for _, txn := range block.Transactions {
		if errCode := tx.VerifyTransaction(txn); errCode != ErrNoError {
			return errors.New("transaction sanity check failed")
		}
		if errCode := tx.VerifyTransactionWithLedger(txn); errCode != ErrNoError {
			return errors.New("transaction history check failed")
		}
	}

	return nil
}

func HeaderCheck(header *Header, ledger *Ledger) error {
	height := header.Height
	if height == 0 {
		return nil
	}
	prevHeader, err := ledger.Blockchain.GetHeader(header.PrevBlockHash)
	if err != nil {
		return errors.New("prev header doesn't exist")
	}
	if prevHeader == nil {
		return errors.New("invalid prev header")
	}
	if prevHeader.Height+1 != height {
		return errors.New("invalid header height")
	}
	timestamp := header.Timestamp
	prevTimestamp := prevHeader.Timestamp
	if timestamp <= prevTimestamp {
		return errors.New("invalid header timestamp")
	}
	if header.WinningHashType == GenesisHash && header.Height >= GenesisBlockProposedHeight {
		return errors.New("invalid winning hash type")
	}

	var miner []byte
	winningHash := prevHeader.WinningHash
	winningHashType := prevHeader.WinningHashType
	switch winningHashType {
	case GenesisHash:
		genesisBlockHash, err := DefaultLedger.Store.GetBlockHash(0)
		if err != nil {
			return err
		}
		genesisBlock, err := DefaultLedger.Store.GetBlock(genesisBlockHash)
		if err != nil {
			return err
		}
		miner, err = genesisBlock.GetSigner()
		if err != nil {
			return err
		}
	case WinningTxnHash:
		if timestamp >= prevTimestamp+int64(config.ProposerChangeTime) {
			return errors.New("expired proposer")
		}
		txn, err := DefaultLedger.Store.GetTransaction(winningHash)
		if err != nil {
			return err
		}
		payload, ok := txn.Payload.(*payload.Commit)
		if !ok {
			return errors.New("invalid transaction type")
		}
		sigchain := &por.SigChain{}
		proto.Unmarshal(payload.SigChain, sigchain)
		miner, err = sigchain.GetMiner()
		if err != nil {
			return err
		}
	case WinningBlockHash:
		timeDiff := time.Duration(timestamp - prevTimestamp)
		if timeDiff < config.ProposerChangeTime {
			return fmt.Errorf("invalid block timestamp, time span with last block: %v", timeDiff)
		}
		index := timeDiff / config.ProposerChangeTime
		proposerBlockHeight := DefaultLedger.Store.GetHeight() - uint32(index)
		proposerBlockHash, err := DefaultLedger.Store.GetBlockHash(proposerBlockHeight)
		if err != nil {
			return err
		}
		proposerBlock, err := DefaultLedger.Store.GetBlock(proposerBlockHash)
		if err != nil {
			return err
		}
		miner, err = proposerBlock.GetSigner()
		if err != nil {
			return err
		}
	case NilHash:
		if timestamp >= prevTimestamp+int64(config.ProposerChangeTime) {
			return errors.New("expired proposer")
		}
		miner = prevHeader.Signer
	}
	if bytes.Compare(miner, header.Signer) != 0 {
		return errors.New("invalid block signer")
	}
	rawPubKey, err := crypto.DecodePoint(miner)
	if err != nil {
		return err
	}
	err = crypto.Verify(*rawPubKey, signature.GetHashForSigning(header), header.Signature)
	if err != nil {
		log.Error("block header verification error: ", err)
		return err
	}

	return nil
}

package ledger

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/signature"
	tx "github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/config"
)

const (
	TimestampTolerance = 40 * time.Second
)

type VBlock struct {
	Block       *Block
	ReceiveTime int64
}

type TransactionArray []*tx.Transaction

func (iterable TransactionArray) Iterate(handler func(item *tx.Transaction) ErrCode) ErrCode {
	for _, item := range iterable {
		result := handler(item)
		if result != ErrNoError {
			return result
		}
	}

	return ErrNoError
}

func TransactionCheck(block *Block) error {
	if block.Transactions == nil {
		return errors.New("empty block")
	}
	if block.Transactions[0].TxType != tx.Coinbase {
		return errors.New("first transaction in block is not Coinbase")
	}
	for i, txn := range block.Transactions {
		if i != 0 && txn.TxType == tx.Coinbase {
			return errors.New("Coinbase transaction order is incorrect")
		}
		if errCode := tx.VerifyTransaction(txn); errCode != ErrNoError {
			return errors.New("transaction sanity check failed")
		}
		if errCode := tx.VerifyTransactionWithLedger(txn); errCode != ErrNoError {
			return errors.New("transaction history check failed")
		}
	}
	if errCode := tx.VerifyTransactionWithBlock(TransactionArray(block.Transactions)); errCode != ErrNoError {
		return errors.New("transaction block check failed")
	}

	return nil
}

// GetNextBlockSigner gets the next block signer after block height at
// timestamp. Returns next signer's public key, chord ID, winner type, and error
func GetNextBlockSigner(height uint32, timestamp int64) ([]byte, []byte, WinnerType, error) {
	currentHeight := DefaultLedger.Store.GetHeight()
	if height > currentHeight {
		return nil, nil, 0, fmt.Errorf("Height %d is higher than current height %d", height, currentHeight)
	}

	headerHash := DefaultLedger.Store.GetHeaderHashByHeight(height)
	header, err := DefaultLedger.Store.GetHeader(headerHash)
	if err != nil {
		return nil, nil, 0, err
	}

	var publicKey []byte
	var chordID []byte
	winnerType := header.WinnerType

	if winnerType == GenesisSigner {
		genesisBlockHash, err := DefaultLedger.Store.GetBlockHash(0)
		if err != nil {
			return nil, nil, 0, err
		}

		genesisBlock, err := DefaultLedger.Store.GetBlock(genesisBlockHash)
		if err != nil {
			return nil, nil, 0, err
		}

		publicKey, chordID, err = genesisBlock.GetSigner()
		if err != nil {
			return nil, nil, 0, err
		}

		return publicKey, chordID, GenesisSigner, nil
	}

	if timestamp <= header.Timestamp {
		return nil, nil, 0, fmt.Errorf("timestamp %d is earlier than previous block timestamp %d", timestamp, header.Timestamp)
	}

	// calculate time difference
	timeSinceLastBlock := timestamp - header.Timestamp
	timeSlot := int64(config.ProposerChangeTime / time.Second)

	if timeSinceLastBlock >= timeSlot {
		winnerType = BlockSigner

		// This is a temporary solution
		proposerBlockHeight := 0
		// index := timeSinceLastBlock / timeSlot
		// proposerBlockHeight := int64(DefaultLedger.Store.GetHeight()) - index
		// if proposerBlockHeight < 0 {
		// proposerBlockHeight = 0
		// }

		proposerBlockHash, err := DefaultLedger.Store.GetBlockHash(uint32(proposerBlockHeight))
		if err != nil {
			return nil, nil, 0, err
		}
		proposerBlock, err := DefaultLedger.Store.GetBlock(proposerBlockHash)
		if err != nil {
			return nil, nil, 0, err
		}
		publicKey, chordID, err = proposerBlock.GetSigner()
		if err != nil {
			return nil, nil, 0, err
		}
	} else {
		switch winnerType {
		case TxnSigner:
			txn, err := DefaultLedger.Store.GetTransaction(header.WinnerHash)
			if err != nil {
				return nil, nil, 0, err
			}
			payload, ok := txn.Payload.(*payload.Commit)
			if !ok {
				return nil, nil, 0, errors.New("invalid transaction type")
			}
			sigchain := &por.SigChain{}
			proto.Unmarshal(payload.SigChain, sigchain)
			publicKey, chordID, err = sigchain.GetMiner()
			if err != nil {
				return nil, nil, 0, err
			}
		case BlockSigner:
		}
	}

	return publicKey, chordID, winnerType, nil
}

// GetWinner returns the winner hash and winner type of a block height using
// sigchain from PoR server.
func GetNextMiningSigChainTxnHash(height uint32) (Uint256, WinnerType, error) {
	if height < NumGenesisBlocks {
		return EmptyUint256, GenesisSigner, nil
	}

	nextMiningSigChainTxnHash, err := por.GetPorServer().GetMiningSigChainTxnHash(height + 1)
	if err != nil {
		return EmptyUint256, TxnSigner, err
	}

	if nextMiningSigChainTxnHash == EmptyUint256 {
		return EmptyUint256, BlockSigner, nil
	}

	return nextMiningSigChainTxnHash, TxnSigner, nil
}

func SignerCheck(header *Header) error {
	currentHeight := DefaultLedger.Store.GetHeight()
	publicKey, chordID, _, err := GetNextBlockSigner(currentHeight, header.Timestamp)
	if err != nil {
		return err
	}

	if !bytes.Equal(header.Signer, publicKey) {
		return fmt.Errorf("invalid block signer public key %x, should be %x", header.Signer, publicKey)
	}

	if len(chordID) > 0 && !bytes.Equal(header.ChordID, chordID) {
		return fmt.Errorf("invalid block signer chord ID %x, should be %x", header.ChordID, chordID)
	}

	rawPubKey, err := crypto.DecodePoint(publicKey)
	if err != nil {
		return err
	}
	err = crypto.Verify(*rawPubKey, signature.GetHashForSigning(header), header.Signature)
	if err != nil {
		return err
	}

	return nil
}

func HeaderCheck(header *Header) error {
	if header.Height == 0 {
		return nil
	}

	expectedHeight := DefaultLedger.Store.GetHeight() + 1
	if header.Height != expectedHeight {
		return fmt.Errorf("Block height %d is different from expected height %d", header.Height, expectedHeight)
	}

	err := SignerCheck(header)
	if err != nil {
		return err
	}

	currentHash := DefaultLedger.Store.GetCurrentBlockHash()
	if header.PrevBlockHash != currentHash {
		return errors.New("invalid prev header")
	}

	prevHeader, err := DefaultLedger.Blockchain.GetHeader(currentHash)
	if err != nil {
		return err
	}
	if prevHeader == nil {
		return errors.New("cannot get prev header")
	}

	if prevHeader.Timestamp >= header.Timestamp {
		return errors.New("invalid header timestamp")
	}

	if header.WinnerType == GenesisSigner && header.Height >= NumGenesisBlocks {
		return errors.New("invalid winning hash type")
	}

	return nil
}

func TimestampCheck(timestamp int64) error {
	t := time.Unix(timestamp, 0) // Handle negative
	now := time.Now()
	earliest := now.Add(-TimestampTolerance)
	latest := now.Add(TimestampTolerance)

	if t.Before(earliest) || t.After(latest) {
		return fmt.Errorf("timestamp %d exceed my tolerance [%d, %d]", timestamp, earliest.Unix(), latest.Unix())
	}

	return nil
}

func NextBlockProposerCheck(block *Block) error {
	winnerHash, winnerType, err := GetNextMiningSigChainTxnHash(block.Header.Height)
	if err != nil {
		return err
	}

	if winnerHash == EmptyUint256 && block.Header.WinnerHash != EmptyUint256 {
		for _, txn := range block.Transactions {
			if txn.Hash() == block.Header.WinnerHash {
				_, err = por.NewPorPackage(txn)
				return err
			}
		}
		return fmt.Errorf("mining sigchain txn %s not found in block", block.Header.WinnerHash.ToHexString())
	}

	if winnerType != block.Header.WinnerType {
		return fmt.Errorf("Winner type should be %v instead of %v", winnerType, block.Header.WinnerType)
	}

	if winnerHash != block.Header.WinnerHash {
		return fmt.Errorf("Winner hash should be %s instead of %s", winnerHash.ToHexString(), block.Header.WinnerHash.ToHexString())
	}

	if block.Header.WinnerHash != EmptyUint256 {
		found := false
		for _, txn := range block.Transactions {
			if txn.Hash() == block.Header.WinnerHash {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("mining sigchain txn %s not found in block", block.Header.WinnerHash.ToHexString())
		}
	}

	return nil
}

func CanVerifyHeight(height uint32) bool {
	return height == DefaultLedger.Store.GetHeight()+1
}

package core

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/signature"
	"github.com/nknorg/nkn/types"
	"github.com/nknorg/nkn/util/config"
)

const (
	TimestampTolerance = 40 * time.Second
	NumGenesisBlocks   = por.SigChainMiningHeightOffset + por.SigChainBlockHeightOffset - 1
	HeaderVersion      = 1
)

type VBlock struct {
	Block       *types.Block
	ReceiveTime int64
}

type TransactionArray []*types.Transaction

func (iterable TransactionArray) Iterate(handler func(item *types.Transaction) ErrCode) ErrCode {
	for _, item := range iterable {
		result := handler(item)
		if result != ErrNoError {
			return result
		}
	}

	return ErrNoError
}

func TransactionCheck(block *types.Block) error {
	//if block.Transactions == nil {
	//	return errors.New("empty block")
	//}
	//if block.Transactions[0].TxType != tx.Coinbase {
	//	return errors.New("first transaction in block is not Coinbase")
	//}
	//for i, txn := range block.Transactions {
	//	if i != 0 && txn.TxType == tx.Coinbase {
	//		return errors.New("Coinbase transaction order is incorrect")
	//	}
	//	if errCode := tx.VerifyTransaction(txn); errCode != ErrNoError {
	//		return errors.New("transaction sanity check failed")
	//	}
	//	if errCode := tx.VerifyTransactionWithLedger(txn); errCode != ErrNoError {
	//		return errors.New("transaction history check failed")
	//	}
	//}
	//if errCode := tx.VerifyTransactionWithBlock(TransactionArray(block.Transactions)); errCode != ErrNoError {
	//	return errors.New("transaction block check failed")
	//}

	return nil
}

// GetNextBlockSigner gets the next block signer after block height at
// timestamp. Returns next signer's public key, chord ID, winner type, and error
func GetNextBlockSigner(height uint32, timestamp int64) ([]byte, []byte, types.WinnerType, error) {
	currentHeight := DefaultLedger.Store.GetHeight()
	if height > currentHeight {
		return nil, nil, 0, fmt.Errorf("Height %d is higher than current height %d", height, currentHeight)
	}

	headerHash := DefaultLedger.Store.GetHeaderHashByHeight(height)
	header, err := DefaultLedger.Store.GetHeader(headerHash)
	if err != nil {
		return nil, nil, 0, err
	}

	// calculate time difference
	var timeDiff int64
	genesisBlockHash, err := DefaultLedger.Store.GetBlockHash(0)
	if err != nil {
		return nil, nil, 0, err
	}
	genesisBlock, err := DefaultLedger.Store.GetBlock(genesisBlockHash)
	if err != nil {
		return nil, nil, 0, err
	}
	prevTimestamp := header.UnsignedHeader.Timestamp
	if prevTimestamp == genesisBlock.Header.UnsignedHeader.Timestamp {
		timeDiff = 0
	} else {
		timeDiff = timestamp - prevTimestamp
	}

	// get miner who will sign next block
	var publicKey []byte
	var chordID []byte
	var winnerType types.WinnerType

	timeSlot := int64(config.ProposerChangeTime / time.Second)

	if timeDiff >= timeSlot {
		winnerType = types.BlockSigner

		// This is a temporary solution
		proposerBlockHeight := 0
		// index := timeDiff / timeSlot
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
		winnerHash, _ := Uint256ParseFromBytes(header.UnsignedHeader.WinnerHash)
		winnerType = header.UnsignedHeader.WinnerType
		switch winnerType {
		case types.GenesisSigner:
			publicKey, chordID, err = genesisBlock.GetSigner()
			if err != nil {
				return nil, nil, 0, err
			}
		case types.TxnSigner:
			txn, err := DefaultLedger.Store.GetTransaction(winnerHash)
			if err != nil {
				return nil, nil, 0, err
			}

			if txn.UnsignedTx.Payload.Type != types.CommitType {
				return nil, nil, 0, errors.New("invalid transaction type")
			}
			commit, err := types.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				return nil, nil, 0, errors.New("invalid payload type")
			}

			payload := commit.(*types.Commit)
			sigchain := &por.SigChain{}
			proto.Unmarshal(payload.SigChain, sigchain)
			publicKey, chordID, err = sigchain.GetMiner()
			if err != nil {
				return nil, nil, 0, err
			}
		}
	}

	return publicKey, chordID, winnerType, nil
}

// GetWinner returns the winner hash and winner type of a block height using
// sigchain from PoR server.
func GetWinner(height uint32) (Uint256, types.WinnerType, error) {
	if height < NumGenesisBlocks {
		return EmptyUint256, types.GenesisSigner, nil
	}

	nextMiningSigChainTxnHash, err := por.GetPorServer().GetMiningSigChainTxnHash(height + 1)
	if err != nil {
		return EmptyUint256, types.TxnSigner, err
	}

	if nextMiningSigChainTxnHash == EmptyUint256 {
		return EmptyUint256, types.BlockSigner, nil
	}

	return nextMiningSigChainTxnHash, types.TxnSigner, nil
}

func SignerCheck(header *types.Header) error {
	currentHeight := DefaultLedger.Store.GetHeight()
	publicKey, chordID, _, err := GetNextBlockSigner(currentHeight, header.UnsignedHeader.Timestamp)
	if err != nil {
		return err
	}

	if !bytes.Equal(header.UnsignedHeader.Signer, publicKey) {
		return fmt.Errorf("invalid block signer public key %x, should be %x", header.UnsignedHeader.Signer, publicKey)
	}

	if len(chordID) > 0 && !bytes.Equal(header.UnsignedHeader.ChordID, chordID) {
		return fmt.Errorf("invalid block signer chord ID %x, should be %x", header.UnsignedHeader.ChordID, chordID)
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

func HeaderCheck(header *types.Header) error {
	if header.UnsignedHeader.Height == 0 {
		return nil
	}

	expectedHeight := DefaultLedger.Store.GetHeight() + 1
	if header.UnsignedHeader.Height != expectedHeight {
		return fmt.Errorf("Block height %d is different from expected height %d", header.UnsignedHeader.Height, expectedHeight)
	}

	err := SignerCheck(header)
	if err != nil {
		return err
	}

	currentHash := DefaultLedger.Store.GetCurrentBlockHash()
	prevHash, _ := Uint256ParseFromBytes(header.UnsignedHeader.PrevBlockHash)
	if prevHash != currentHash {
		return errors.New("invalid prev header")
	}

	prevHeader, err := DefaultLedger.Blockchain.GetHeader(currentHash)
	if err != nil {
		return err
	}
	if prevHeader == nil {
		return errors.New("cannot get prev header")
	}

	if prevHeader.UnsignedHeader.Timestamp >= header.UnsignedHeader.Timestamp {
		return errors.New("invalid header timestamp")
	}

	if header.UnsignedHeader.WinnerType == types.GenesisSigner && header.UnsignedHeader.Height >= NumGenesisBlocks {
		return errors.New("invalid winning hash type")
	}

	return nil
}

func TimestampCheck(timestamp int64) error {
	now := time.Now()
	earliest := now.Add(-TimestampTolerance).Unix()
	latest := now.Add(TimestampTolerance).Unix()
	if timestamp < earliest || timestamp > latest {
		return errors.New("Invalid timestamp")
	}
	return nil
}

func CanVerifyHeight(height uint32) bool {
	return height == DefaultLedger.Store.GetHeight()+1
}

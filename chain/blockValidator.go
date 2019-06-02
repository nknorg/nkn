package chain

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/block"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vm/signature"
)

const (
	TimestampTolerance = 40 * time.Second
	NumGenesisBlocks   = por.SigChainMiningHeightOffset + config.MaxRollbackBlocks - 1
	HeaderVersion      = 1
)

type VBlock struct {
	Block       *block.Block
	ReceiveTime int64
}

type TransactionArray []*transaction.Transaction

func (iterable TransactionArray) Iterate(handler func(item *transaction.Transaction) error) error {
	for _, item := range iterable {
		result := handler(item)
		if result != nil {
			return result
		}
	}

	return nil
}

func TransactionCheck(block *block.Block) error {
	if block.Transactions == nil {
		return errors.New("empty block")
	}
	numTxn := len(block.Transactions)
	if numTxn > config.MaxNumTxnPerBlock {
		return errors.New("block contains too many transactions")
	}

	blockSize := block.GetTxsSize()
	if blockSize > config.MaxBlockSize {
		return errors.New("serialized block is too big")
	}

	if block.Transactions[0].UnsignedTx.Payload.Type != pb.CoinbaseType {
		return errors.New("first transaction in block is not Coinbase")
	}

	winnerHash, err := Uint256ParseFromBytes(block.Header.UnsignedHeader.WinnerHash)
	if err != nil {
		return err
	}
	if winnerHash != EmptyUint256 {
		found := false
		for _, txn := range block.Transactions {
			if txn.Hash() == winnerHash {
				found = true
				break
			}
		}
		if !found {
			if _, err = DefaultLedger.Store.GetTransaction(winnerHash); err != nil {
				return fmt.Errorf("mining sigchain txn %s not found in block", winnerHash)
			}
		}
	}

	txnsHash := make([]Uint256, len(block.Transactions))
	for i, txn := range block.Transactions {
		txnsHash[i] = txn.Hash()
	}
	txnsRoot, err := crypto.ComputeRoot(txnsHash)
	if err != nil {
		return fmt.Errorf("compute txns root error: %v", err)
	}
	if !bytes.Equal(txnsRoot.ToArray(), block.Header.UnsignedHeader.TransactionsRoot) {
		return fmt.Errorf("computed txn root %x is different from txn root in header %x, fall back to request full txn hash", txnsRoot.ToArray(), block.Header.UnsignedHeader.TransactionsRoot)
	}

	nonces := make(map[Uint160]uint64, 0)
	for i, txn := range block.Transactions {
		if i != 0 && txn.UnsignedTx.Payload.Type == pb.CoinbaseType {
			return errors.New("Coinbase transaction order is incorrect")
		}
		if err := VerifyTransaction(txn); err != nil {
			return fmt.Errorf("transaction sanity check failed: %v", err)
		}
		if err := VerifyTransactionWithLedger(txn); err != nil {
			return fmt.Errorf("transaction history check failed: %v", err)
		}

		if txn.UnsignedTx.Payload.Type != pb.CoinbaseType && txn.UnsignedTx.Payload.Type != pb.CommitType {
			addr, err := ToCodeHash(txn.Programs[0].Code)
			if err != nil {
				return err
			}

			if _, ok := nonces[addr]; !ok {
				nonce := DefaultLedger.Store.GetNonce(addr)
				nonces[addr] = nonce
			}

			if nonces[addr] != txn.UnsignedTx.Nonce {
				return fmt.Errorf("txn nonce error, expected: %v, Get: %v", nonces[addr], txn.UnsignedTx.Nonce)
			}

			nonces[addr]++
		}

	}
	if err := VerifyTransactionWithBlock(TransactionArray(block.Transactions), block.Header); err != nil {
		return fmt.Errorf("Transaction block check failed: %v", err)
	}

	//state root check
	root := GenerateStateRoot(block.Transactions, block.Header.UnsignedHeader.Height, block.Header.UnsignedHeader.RandomBeacon)

	headerRoot, _ := Uint256ParseFromBytes(block.Header.UnsignedHeader.StateRoot)
	if ok := root.CompareTo(headerRoot); ok != 0 {
		return fmt.Errorf("[TransactionCheck]state root not equal:%v, %v", root, headerRoot)
	}

	return nil
}

// GetNextBlockSigner gets the next block signer after block height at
// timestamp. Returns next signer's public key, chord ID, winner type, and error
func GetNextBlockSigner(height uint32, timestamp int64) ([]byte, []byte, pb.WinnerType, error) {
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
	winnerType := header.UnsignedHeader.WinnerType

	if winnerType == pb.GENESIS_SIGNER {
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

		return publicKey, chordID, pb.GENESIS_SIGNER, nil
	}

	if timestamp <= header.UnsignedHeader.Timestamp {
		return nil, nil, 0, fmt.Errorf("timestamp %d is earlier than previous block timestamp %d", timestamp, header.UnsignedHeader.Timestamp)
	}

	timeSinceLastBlock := timestamp - header.UnsignedHeader.Timestamp
	proposerChangeTime := int64(config.ConsensusTimeout.Seconds())

	if proposerChangeTime-timeSinceLastBlock%proposerChangeTime <= int64(config.ConsensusDuration.Seconds()) {
		return nil, nil, 0, nil
	}

	if timeSinceLastBlock >= proposerChangeTime {
		winnerType = pb.BLOCK_SIGNER

		proposerBlockHeight := int64(DefaultLedger.Store.GetHeight()) - timeSinceLastBlock/proposerChangeTime
		if proposerBlockHeight < 0 {
			proposerBlockHeight = 0
		}

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
		case pb.TXN_SIGNER:
			whash, _ := Uint256ParseFromBytes(header.UnsignedHeader.WinnerHash)
			txn, err := DefaultLedger.Store.GetTransaction(whash)
			if err != nil {
				return nil, nil, 0, err
			}

			if txn.UnsignedTx.Payload.Type != pb.CommitType {
				return nil, nil, 0, errors.New("invalid transaction type")
			}

			commit, err := transaction.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				return nil, nil, 0, errors.New("invalid payload type")
			}

			payload := commit.(*pb.Commit)
			sigchain := &pb.SigChain{}
			proto.Unmarshal(payload.SigChain, sigchain)
			publicKey, chordID, err = sigchain.GetMiner()
			if err != nil {
				return nil, nil, 0, err
			}
		case pb.BLOCK_SIGNER:
		}
	}

	return publicKey, chordID, winnerType, nil
}

// GetWinner returns the winner hash and winner type of a block height using
// sigchain from PoR server.
func GetNextMiningSigChainTxnHash(height uint32) (Uint256, pb.WinnerType, error) {
	if height < NumGenesisBlocks {
		return EmptyUint256, pb.GENESIS_SIGNER, nil
	}

	nextMiningSigChainTxnHash, err := por.GetPorServer().GetMiningSigChainTxnHash(height + 1)
	if err != nil {
		return EmptyUint256, pb.TXN_SIGNER, err
	}

	if nextMiningSigChainTxnHash == EmptyUint256 {
		return EmptyUint256, pb.BLOCK_SIGNER, nil
	}

	return nextMiningSigChainTxnHash, pb.TXN_SIGNER, nil
}

func SignerCheck(header *block.Header) error {
	currentHeight := DefaultLedger.Store.GetHeight()
	publicKey, chordID, _, err := GetNextBlockSigner(currentHeight, header.UnsignedHeader.Timestamp)
	if err != nil {
		return err
	}

	if !bytes.Equal(header.UnsignedHeader.Signer, publicKey) {
		return fmt.Errorf("invalid block signer public key %x, should be %x", header.UnsignedHeader.Signer, publicKey)
	}

	if len(chordID) > 0 && !bytes.Equal(header.UnsignedHeader.ChordId, chordID) {
		return fmt.Errorf("invalid block signer chord ID %x, should be %x", header.UnsignedHeader.ChordId, chordID)
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

func HeaderCheck(header *block.Header) error {
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

	if len(header.UnsignedHeader.RandomBeacon) != config.RandomBeaconLength {
		return errors.New("invalid header RandomBeacon")
	}

	if prevHeader.UnsignedHeader.Timestamp >= header.UnsignedHeader.Timestamp {
		return errors.New("invalid header timestamp")
	}

	if header.UnsignedHeader.WinnerType == pb.GENESIS_SIGNER && header.UnsignedHeader.Height >= NumGenesisBlocks {
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

func NextBlockProposerCheck(header *block.Header) error {
	expectedWinnerHash, expectedWinnerType, err := GetNextMiningSigChainTxnHash(header.UnsignedHeader.Height)
	if err != nil {
		return err
	}

	winnerType := header.UnsignedHeader.WinnerType
	winnerHash, err := Uint256ParseFromBytes(header.UnsignedHeader.WinnerHash)
	if err != nil {
		return err
	}

	if winnerType != expectedWinnerType {
		return fmt.Errorf("Winner type should be %v instead of %v", expectedWinnerType, winnerType)
	}

	if winnerHash != expectedWinnerHash {
		return fmt.Errorf("Winner hash should be %x instead of %x", expectedWinnerHash, winnerHash)
	}

	return nil
}

func CanVerifyHeight(height uint32) bool {
	return height == DefaultLedger.Store.GetHeight()+1
}

func VerifyHeader(header *block.Header) bool {
	if header.UnsignedHeader.Height != DefaultLedger.Store.GetHeaderHeight()+1 {
		log.Error("[VerifyHeader] failed, header height error.")
		return false
	}
	prevHash, _ := Uint256ParseFromBytes(header.UnsignedHeader.PrevBlockHash)
	prevHeader, err := DefaultLedger.Store.GetHeaderWithCache(prevHash)
	if err != nil || prevHeader == nil {
		log.Error("[VerifyHeader] failed, not found prevHeader.")
		return false
	}

	if prevHeader.UnsignedHeader.Height+1 != header.UnsignedHeader.Height {
		log.Error("[VerifyHeader] failed, prevHeader.Height + 1 != header.Height")
		return false
	}

	if prevHeader.UnsignedHeader.Timestamp >= header.UnsignedHeader.Timestamp {
		log.Error("[VerifyHeader] failed, prevHeader.Timestamp >= header.Timestamp")
		return false
	}

	//	flag, err := signature.VerifySignableData(header)
	//	if flag == false || err != nil {
	//		log.Error("[VerifyHeader] failed, VerifySignableData failed.")
	//		log.Error(err)
	//		return false
	//	}

	return true
}

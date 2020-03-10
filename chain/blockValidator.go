package chain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/signature"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
)

const (
	TimestampToleranceFuture   = config.ConsensusDuration / 4
	TimestampTolerancePast     = config.ConsensusDuration / 2
	TimestampToleranceVariance = config.ConsensusDuration / 6
	ProposingTimeTolerance     = config.ConsensusDuration / 2
	NumGenesisBlocks           = 4
)

var timestampToleranceSalt []byte = util.RandomBytes(32)

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

func TransactionCheck(ctx context.Context, block *block.Block) error {
	if block.IsTxnsChecked {
		return nil
	}

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

	if block.Transactions[0].UnsignedTx.Payload.Type != pb.COINBASE_TYPE {
		return errors.New("first transaction in block is not Coinbase")
	}

	txnsHash := make([]common.Uint256, len(block.Transactions))
	for i, txn := range block.Transactions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		txnsHash[i] = txn.Hash()
	}

	winnerHash, err := common.Uint256ParseFromBytes(block.Header.UnsignedHeader.WinnerHash)
	if err != nil {
		return err
	}
	if winnerHash != common.EmptyUint256 {
		found := false
		for _, txnHash := range txnsHash {
			if txnHash == winnerHash {
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

	txnsRoot, err := crypto.ComputeRoot(txnsHash)
	if err != nil {
		return fmt.Errorf("compute txns root error: %v", err)
	}
	if !bytes.Equal(txnsRoot.ToArray(), block.Header.UnsignedHeader.TransactionsRoot) {
		return fmt.Errorf("computed txn root %x is different from txn root in header %x", txnsRoot.ToArray(), block.Header.UnsignedHeader.TransactionsRoot)
	}

	bvs := NewBlockValidationState()

	nonces := make(map[common.Uint160]uint64, 0)
	for i, txn := range block.Transactions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if i != 0 && txn.UnsignedTx.Payload.Type == pb.COINBASE_TYPE {
			return errors.New("Coinbase transaction order is incorrect")
		}
		if err := VerifyTransaction(txn, block.Header.UnsignedHeader.Height); err != nil {
			return fmt.Errorf("transaction sanity check failed: %v", err)
		}
		if err := bvs.VerifyTransactionWithBlock(txn, block.Header.UnsignedHeader.Height); err != nil {
			bvs.Reset()
			return fmt.Errorf("transaction block check failed: %v", err)
		}
		bvs.Commit()
		if err := VerifyTransactionWithLedger(txn, block.Header.UnsignedHeader.Height); err != nil {
			return fmt.Errorf("transaction history check failed: %v", err)
		}

		switch txn.UnsignedTx.Payload.Type {
		case pb.COINBASE_TYPE:
		case pb.SIG_CHAIN_TXN_TYPE:
		case pb.NANO_PAY_TYPE:
		default:
			addr, err := common.ToCodeHash(txn.Programs[0].Code)
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

	bvs.Close()

	// state root check
	root, err := DefaultLedger.Store.GenerateStateRoot(ctx, block, true, false)
	if err != nil {
		return err
	}

	headerRoot, _ := common.Uint256ParseFromBytes(block.Header.UnsignedHeader.StateRoot)
	if ok := root.CompareTo(headerRoot); ok != 0 {
		return fmt.Errorf("[TransactionCheck]state root not equal:%v, %v", root, headerRoot)
	}

	block.IsTxnsChecked = true

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

	if timeSinceLastBlock >= proposerChangeTime {
		if timeSinceLastBlock%proposerChangeTime > int64(ProposingTimeTolerance.Seconds()) {
			return nil, nil, 0, nil
		}

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
		if timeSinceLastBlock < int64(config.ConsensusDuration.Seconds()) || timeSinceLastBlock > int64(config.ConsensusDuration.Seconds())+int64(ProposingTimeTolerance.Seconds()) {
			return nil, nil, 0, nil
		}

		switch winnerType {
		case pb.TXN_SIGNER:
			whash, _ := common.Uint256ParseFromBytes(header.UnsignedHeader.WinnerHash)
			txn, err := DefaultLedger.Store.GetTransaction(whash)
			if err != nil {
				return nil, nil, 0, err
			}

			if txn.UnsignedTx.Payload.Type != pb.SIG_CHAIN_TXN_TYPE {
				return nil, nil, 0, errors.New("invalid transaction type")
			}

			payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				return nil, nil, 0, err
			}

			sigChainTxn := payload.(*pb.SigChainTxn)
			sigChain := &pb.SigChain{}
			proto.Unmarshal(sigChainTxn.SigChain, sigChain)
			publicKey, chordID, err = sigChain.GetMiner()
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
func GetNextMiningSigChainTxnHash(height uint32) (common.Uint256, pb.WinnerType, error) {
	if height < NumGenesisBlocks {
		return common.EmptyUint256, pb.GENESIS_SIGNER, nil
	}

	nextMiningSigChainTxnHash, err := por.GetPorServer().GetMiningSigChainTxnHash(height + 1)
	if err != nil {
		return common.EmptyUint256, pb.TXN_SIGNER, err
	}

	if nextMiningSigChainTxnHash == common.EmptyUint256 {
		return common.EmptyUint256, pb.BLOCK_SIGNER, nil
	}

	return nextMiningSigChainTxnHash, pb.TXN_SIGNER, nil
}

func SignerCheck(header *block.Header) error {
	currentHeight := DefaultLedger.Store.GetHeight()
	publicKey, chordID, _, err := GetNextBlockSigner(currentHeight, header.UnsignedHeader.Timestamp)
	if err != nil {
		return fmt.Errorf("get next block signer error: %v", err)
	}

	if !bytes.Equal(header.UnsignedHeader.SignerPk, publicKey) {
		return fmt.Errorf("invalid block signer public key %x, should be %x", header.UnsignedHeader.SignerPk, publicKey)
	}

	_, err = crypto.DecodePoint(header.UnsignedHeader.SignerPk)
	if err != nil {
		return fmt.Errorf("decode public key error: %v", err)
	}

	if len(chordID) > 0 && !bytes.Equal(header.UnsignedHeader.SignerId, chordID) {
		return fmt.Errorf("invalid block signer chord ID %x, should be %x", header.UnsignedHeader.SignerId, chordID)
	}

	id, err := DefaultLedger.Store.GetID(publicKey)
	if err != nil {
		return fmt.Errorf("get ID of signer %x error: %v", publicKey, err)
	}
	if len(id) == 0 {
		return fmt.Errorf("ID of signer %x is empty", publicKey)
	}
	if !bytes.Equal(header.UnsignedHeader.SignerId, id) {
		return fmt.Errorf("ID of signer %x should be %x, got %x", publicKey, id, header.UnsignedHeader.SignerId)
	}

	return nil
}

func SignatureCheck(header *block.Header) error {
	rawPubKey, err := crypto.DecodePoint(header.UnsignedHeader.SignerPk)
	if err != nil {
		return fmt.Errorf("decode public key error: %v", err)
	}

	err = crypto.Verify(*rawPubKey, signature.GetHashForSigning(header), header.Signature)
	if err != nil {
		return fmt.Errorf("invalid header signature %x: %v", header.Signature, err)
	}

	return nil
}

func HeaderCheck(b *block.Block) error {
	if b.IsHeaderChecked {
		return nil
	}

	header := b.Header

	if header.UnsignedHeader.Height == 0 {
		return nil
	}

	expectedHeight := DefaultLedger.Store.GetHeight() + 1
	if header.UnsignedHeader.Height != expectedHeight {
		return fmt.Errorf("block height %d is different from expected height %d", header.UnsignedHeader.Height, expectedHeight)
	}

	err := SignerCheck(header)
	if err != nil {
		return fmt.Errorf("signer check failed: %v", err)
	}

	err = SignatureCheck(header)
	if err != nil {
		return fmt.Errorf("signature check failed: %v", err)
	}

	currentHash := DefaultLedger.Store.GetCurrentBlockHash()
	prevHash, err := common.Uint256ParseFromBytes(header.UnsignedHeader.PrevBlockHash)
	if err != nil {
		return fmt.Errorf("parse prev block hash %x error: %v", header.UnsignedHeader.PrevBlockHash, err)
	}
	if prevHash != currentHash {
		return fmt.Errorf("invalid prev header %x, expecting %x", prevHash.ToArray(), currentHash.ToArray())
	}

	prevHeader, err := DefaultLedger.Blockchain.GetHeader(currentHash)
	if err != nil {
		return fmt.Errorf("get header by hash %x error: %v", currentHash.ToArray(), err)
	}
	if prevHeader == nil {
		return fmt.Errorf("cannot get prev header by hash %x", currentHash.ToArray())
	}

	if prevHeader.UnsignedHeader.Timestamp >= header.UnsignedHeader.Timestamp {
		return fmt.Errorf("header timestamp %d is not greater than prev timestamp %d", header.UnsignedHeader.Timestamp, prevHeader.UnsignedHeader.Timestamp)
	}

	if header.UnsignedHeader.WinnerType == pb.GENESIS_SIGNER && header.UnsignedHeader.Height >= NumGenesisBlocks {
		return fmt.Errorf("invalid winner type %v for height %d", pb.GENESIS_SIGNER, header.UnsignedHeader.Height)
	}

	if len(header.UnsignedHeader.RandomBeacon) != config.RandomBeaconLength {
		return fmt.Errorf("invalid header RandomBeacon length %d, expecting %d", len(header.UnsignedHeader.RandomBeacon), config.RandomBeaconLength)
	}

	rawPubKey, err := crypto.DecodePoint(header.UnsignedHeader.SignerPk)
	if err != nil {
		return err
	}
	vrf := header.UnsignedHeader.RandomBeacon[:config.RandomBeaconUniqueLength]
	proof := header.UnsignedHeader.RandomBeacon[config.RandomBeaconUniqueLength:]
	prevVrf := prevHeader.UnsignedHeader.RandomBeacon[:config.RandomBeaconUniqueLength]
	if !crypto.VerifyVrf(*rawPubKey, prevVrf, vrf, proof) {
		return fmt.Errorf("invalid header RandomBeacon %x", header.UnsignedHeader.RandomBeacon)
	}

	b.IsHeaderChecked = true

	return nil
}

func TimestampCheck(header *block.Header, soft bool) error {
	prevHeader, err := DefaultLedger.Store.GetHeaderByHeight(header.UnsignedHeader.Height - 1)
	if err != nil {
		return err
	}

	t := time.Unix(header.UnsignedHeader.Timestamp, 0) // Handle negative

	prevBlockTimestamp := time.Unix(prevHeader.UnsignedHeader.Timestamp, 0)
	if t.Unix() <= prevBlockTimestamp.Unix() {
		return fmt.Errorf("block timestamp %d is not later than previous block timestamp %d", t.Unix(), prevBlockTimestamp.Unix())
	}

	now := time.Now()
	earliest := now.Add(-TimestampTolerancePast)
	latest := now.Add(TimestampToleranceFuture)

	if soft {
		h := fnv.New64()
		blockHash := header.Hash()
		h.Write(blockHash.ToArray())
		h.Write(timestampToleranceSalt)
		offsetSec := int64(h.Sum64()%uint64(2*TimestampToleranceVariance.Seconds()+1)) - int64(TimestampToleranceVariance.Seconds())
		offset := time.Duration(offsetSec) * time.Second
		earliest = earliest.Add(offset)
	} else {
		earliest = earliest.Add(-TimestampToleranceVariance)
	}

	if t.Unix() < earliest.Unix() || t.Unix() > latest.Unix() {
		return fmt.Errorf("block timestamp %d exceed my tolerance [%d, %d]", t.Unix(), earliest.Unix(), latest.Unix())
	}

	return nil
}

func NextBlockProposerCheck(header *block.Header) error {
	expectedWinnerHash, expectedWinnerType, err := GetNextMiningSigChainTxnHash(header.UnsignedHeader.Height)
	if err != nil {
		return err
	}

	winnerType := header.UnsignedHeader.WinnerType
	winnerHash, err := common.Uint256ParseFromBytes(header.UnsignedHeader.WinnerHash)
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
	prevHash, _ := common.Uint256ParseFromBytes(header.UnsignedHeader.PrevBlockHash)
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

	return true
}

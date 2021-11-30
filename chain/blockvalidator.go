package chain

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/nknorg/nkn/v2/chain/txvalidator"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/block"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/por"
	"github.com/nknorg/nkn/v2/program"
	"github.com/nknorg/nkn/v2/transaction"
	"github.com/nknorg/nkn/v2/util"
)

const (
	TimestampToleranceFuture   = config.ConsensusDuration / 4
	TimestampTolerancePast     = config.ConsensusDuration / 2
	TimestampToleranceVariance = config.ConsensusDuration / 6
	ProposingTimeTolerance     = config.ConsensusDuration / 2
	NumGenesisBlocks           = 4
)

var (
	ErrIDRegistered           = errors.New("id has been registered")
	ErrDuplicateGenerateIDTxn = errors.New("duplicate GenerateID txns")
	ErrDuplicateIssueAssetTxn = errors.New("duplicate IssueAsset txns")
	timestampToleranceSalt    = util.RandomBytes(32)
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

func TransactionCheck(ctx context.Context, block *block.Block, fastSync bool) error {
	if block.IsTxnsChecked {
		return nil
	}

	if len(block.Transactions) == 0 {
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

	if block.Transactions[0].UnsignedTx.Payload.Type != pb.PayloadType_COINBASE_TYPE {
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

	txnsRoot, err := crypto.ComputeRoot(txnsHash)
	if err != nil {
		return fmt.Errorf("compute txns root error: %v", err)
	}
	if !bytes.Equal(txnsRoot.ToArray(), block.Header.UnsignedHeader.TransactionsRoot) {
		return fmt.Errorf("computed txn root %x is different from txn root in header %x", txnsRoot.ToArray(), block.Header.UnsignedHeader.TransactionsRoot)
	}

	if fastSync {
		for _, txn := range block.Transactions {
			err = txn.VerifySignature()
			if err != nil {
				return fmt.Errorf("[VerifyTransaction] %v", err)
			}
		}
		return nil
	}

	bvs := NewBlockValidationState()

	nonces := make(map[common.Uint160]uint64, 0)
	for i, txn := range block.Transactions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if i != 0 && txn.UnsignedTx.Payload.Type == pb.PayloadType_COINBASE_TYPE {
			return errors.New("coinbase transaction order is incorrect")
		}
		if err := txvalidator.VerifyTransaction(txn, block.Header.UnsignedHeader.Height); err != nil {
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
		case pb.PayloadType_COINBASE_TYPE:
		case pb.PayloadType_SIG_CHAIN_TXN_TYPE:
		case pb.PayloadType_NANO_PAY_TYPE:
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
				return fmt.Errorf("mining sigchain txn %s not found in block", winnerHash.ToHexString())
			}
		}
	}

	// state root check
	root, err := DefaultLedger.Store.GenerateStateRoot(ctx, block, true, false)
	if err != nil {
		return err
	}

	headerRoot, _ := common.Uint256ParseFromBytes(block.Header.UnsignedHeader.StateRoot)
	if ok := root.CompareTo(headerRoot); ok != 0 {
		return fmt.Errorf("[TransactionCheck]state root not equal:%v, %v", root.ToHexString(), headerRoot.ToString())
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

	if winnerType == pb.WinnerType_GENESIS_SIGNER {
		genesisBlockHash, err := DefaultLedger.Store.GetBlockHash(0)
		if err != nil {
			return nil, nil, 0, err
		}

		genesisBlock, err := DefaultLedger.Store.GetBlock(genesisBlockHash)
		if err != nil {
			return nil, nil, 0, err
		}

		publicKey, chordID, err = genesisBlock.Header.GetSigner()
		if err != nil {
			return nil, nil, 0, err
		}

		return publicKey, chordID, pb.WinnerType_GENESIS_SIGNER, nil
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

		winnerType = pb.WinnerType_BLOCK_SIGNER

		proposerBlockHeight := int64(DefaultLedger.Store.GetHeight()) - timeSinceLastBlock/proposerChangeTime
		if proposerBlockHeight < 0 {
			proposerBlockHeight = 0
		}

		proposerBlockHash, err := DefaultLedger.Store.GetBlockHash(uint32(proposerBlockHeight))
		if err != nil {
			return nil, nil, 0, err
		}
		proposerBlockHeader, err := DefaultLedger.Store.GetHeader(proposerBlockHash)
		if err != nil {
			return nil, nil, 0, err
		}
		publicKey, chordID, err = proposerBlockHeader.GetSigner()
		if err != nil {
			return nil, nil, 0, err
		}
	} else {
		if timeSinceLastBlock < int64(config.ConsensusDuration.Seconds()) || timeSinceLastBlock > int64(config.ConsensusDuration.Seconds())+int64(ProposingTimeTolerance.Seconds()) {
			return nil, nil, 0, nil
		}

		switch winnerType {
		case pb.WinnerType_TXN_SIGNER:
			whash, err := common.Uint256ParseFromBytes(header.UnsignedHeader.WinnerHash)
			if err != nil {
				return nil, nil, 0, err
			}

			txn, err := DefaultLedger.Store.GetTransaction(whash)
			if err != nil {
				return nil, nil, 0, err
			}

			if txn.UnsignedTx.Payload.Type != pb.PayloadType_SIG_CHAIN_TXN_TYPE {
				return nil, nil, 0, errors.New("invalid transaction type")
			}

			payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				return nil, nil, 0, err
			}

			sigChainTxn := payload.(*pb.SigChainTxn)
			sigChain := &pb.SigChain{}
			err = proto.Unmarshal(sigChainTxn.SigChain, sigChain)
			if err != nil {
				return nil, nil, 0, err
			}

			blockHash, err := common.Uint256ParseFromBytes(sigChain.BlockHash)
			if err != nil {
				return nil, nil, 0, err
			}

			height, err := DefaultLedger.Store.GetHeightByBlockHash(blockHash)
			if err != nil {
				return nil, nil, 0, err
			}

			publicKey, chordID, err = sigChain.GetMiner(height, header.UnsignedHeader.RandomBeacon)
			if err != nil {
				return nil, nil, 0, err
			}
		case pb.WinnerType_BLOCK_SIGNER:
		}
	}

	return publicKey, chordID, winnerType, nil
}

// GetNextMiningSigChainTxnHash returns the winner hash and winner type of a block height using
// sigchain from PoR server.
func GetNextMiningSigChainTxnHash(height uint32) (common.Uint256, pb.WinnerType, error) {
	if height < NumGenesisBlocks {
		return common.EmptyUint256, pb.WinnerType_GENESIS_SIGNER, nil
	}

	nextMiningSigChainTxnHash, err := por.GetPorServer().GetMiningSigChainTxnHash(height + 1)
	if err != nil {
		return common.EmptyUint256, pb.WinnerType_TXN_SIGNER, err
	}

	if nextMiningSigChainTxnHash == common.EmptyUint256 {
		return common.EmptyUint256, pb.WinnerType_BLOCK_SIGNER, nil
	}

	return nextMiningSigChainTxnHash, pb.WinnerType_TXN_SIGNER, nil
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

	err = crypto.CheckPublicKey(header.UnsignedHeader.SignerPk)
	if err != nil {
		return fmt.Errorf("invalid public key: %v", err)
	}

	if len(chordID) > 0 && !bytes.Equal(header.UnsignedHeader.SignerId, chordID) {
		return fmt.Errorf("invalid block signer chord ID %x, should be %x", header.UnsignedHeader.SignerId, chordID)
	}
	id, err := DefaultLedger.Store.GetID(publicKey, currentHeight)
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

func HeaderCheck(header *block.Header, fastSync bool) error {
	if header.IsHeaderChecked {
		return nil
	}

	if header.UnsignedHeader.Height == 0 {
		return nil
	}

	expectedHeight := DefaultLedger.Store.GetHeight() + 1
	if !fastSync && header.UnsignedHeader.Height != expectedHeight {
		return fmt.Errorf("block height %d is different from expected height %d", header.UnsignedHeader.Height, expectedHeight)
	}

	err := header.VerifySignature()
	if err != nil {
		return fmt.Errorf("invalid header signature %x: %v", header.Signature, err)
	}

	if fastSync {
		return nil
	}

	err = SignerCheck(header)
	if err != nil {
		return fmt.Errorf("signer check failed: %v", err)
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

	if header.UnsignedHeader.WinnerType == pb.WinnerType_GENESIS_SIGNER && header.UnsignedHeader.Height >= NumGenesisBlocks {
		return fmt.Errorf("invalid winner type %v for height %d", pb.WinnerType_GENESIS_SIGNER, header.UnsignedHeader.Height)
	}

	if len(header.UnsignedHeader.RandomBeacon) != config.RandomBeaconLength {
		return fmt.Errorf("invalid header RandomBeacon length %d, expecting %d", len(header.UnsignedHeader.RandomBeacon), config.RandomBeaconLength)
	}

	vrf := header.UnsignedHeader.RandomBeacon[:config.RandomBeaconUniqueLength]
	proof := header.UnsignedHeader.RandomBeacon[config.RandomBeaconUniqueLength:]
	prevVrf := prevHeader.UnsignedHeader.RandomBeacon[:config.RandomBeaconUniqueLength]
	if !crypto.VerifyVrf(header.UnsignedHeader.SignerPk, prevVrf, vrf, proof) {
		return fmt.Errorf("invalid header RandomBeacon %x", header.UnsignedHeader.RandomBeacon)
	}

	header.IsHeaderChecked = true

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

// VerifyTransactionWithLedger verifys a transaction with history transaction in ledger
func VerifyTransactionWithLedger(txn *transaction.Transaction, height uint32) error {
	if DefaultLedger.Store.IsDoubleSpend(txn) {
		return errors.New("[VerifyTransactionWithLedger] IsDoubleSpend check faild")
	}

	if DefaultLedger.Store.IsTxHashDuplicate(txn.Hash()) {
		return errors.New("[VerifyTransactionWithLedger] duplicate transaction check faild")
	}

	payload, err := transaction.Unpack(txn.UnsignedTx.Payload)
	if err != nil {
		return errors.New("unpack transactiion's payload error")
	}

	pg, err := txn.GetProgramHashes()
	if err != nil {
		return err
	}

	switch txn.UnsignedTx.Payload.Type {
	case pb.PayloadType_NANO_PAY_TYPE:
	case pb.PayloadType_SIG_CHAIN_TXN_TYPE:
	default:
		if txn.UnsignedTx.Nonce < DefaultLedger.Store.GetNonce(pg[0]) {
			return errors.New("nonce is too low")
		}
	}

	var amount int64

	switch txn.UnsignedTx.Payload.Type {
	case pb.PayloadType_COINBASE_TYPE:
		donationAmount, err := DefaultLedger.Store.GetDonation()
		if err != nil {
			return err
		}

		donationProgramhash, err := common.ToScriptHash(config.DonationAddress)
		if err != nil {
			return err
		}

		donationBalance := DefaultLedger.Store.GetBalance(donationProgramhash)
		if donationBalance < donationAmount {
			return errors.New("not sufficient funds in donation account")
		}
	case pb.PayloadType_TRANSFER_ASSET_TYPE:
		pld := payload.(*pb.TransferAsset)
		amount += pld.Amount
	case pb.PayloadType_SIG_CHAIN_TXN_TYPE:
	case pb.PayloadType_REGISTER_NAME_TYPE:
		pld := payload.(*pb.RegisterName)
		if config.LegacyNameService.GetValueAtHeight(height) {
			name, err := DefaultLedger.Store.GetName_legacy(pld.Registrant)
			if name != "" {
				return fmt.Errorf("pubKey %s already has registered name %s", hex.EncodeToString(pld.Registrant), name)
			}
			if err != nil {
				return err
			}

			registrant, err := DefaultLedger.Store.GetRegistrant_legacy(pld.Name)
			if err != nil {
				return err
			}
			if registrant != nil {
				return fmt.Errorf("name %s is already registered for pubKey %+v", pld.Name, registrant)
			}
		} else {
			registrant, _, err := DefaultLedger.Store.GetRegistrant(pld.Name)
			if err != nil {
				return err
			}
			if len(registrant) > 0 && !bytes.Equal(registrant, pld.Registrant) {
				return fmt.Errorf("name %s is already registered for pubKey %+v", pld.Name, registrant)
			}
			amount += pld.RegistrationFee
		}
	case pb.PayloadType_TRANSFER_NAME_TYPE:
		pld := payload.(*pb.TransferName)

		registrant, _, err := DefaultLedger.Store.GetRegistrant(pld.Name)
		if err != nil {
			return err
		}
		if len(registrant) == 0 {
			return fmt.Errorf("can not transfer unregistered name")
		}
		if bytes.Equal(registrant, pld.Recipient) {
			return fmt.Errorf("can not transfer names to its owner")
		}
		if !bytes.Equal(registrant, pld.Registrant) {
			return fmt.Errorf("registrant incorrect")
		}
		senderPubkey, err := program.GetPublicKeyFromCode(txn.Programs[0].Code)
		if err != nil {
			return err
		}
		if !bytes.Equal(registrant, senderPubkey) {
			return fmt.Errorf("can not transfer names which did not belongs to you")
		}

	case pb.PayloadType_DELETE_NAME_TYPE:
		pld := payload.(*pb.DeleteName)
		if config.LegacyNameService.GetValueAtHeight(height) {
			name, err := DefaultLedger.Store.GetName_legacy(pld.Registrant)
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("no name registered for pubKey %+v", pld.Registrant)
			} else if name != pld.Name {
				return fmt.Errorf("no name %s registered for pubKey %+v", pld.Name, pld.Registrant)
			}
		} else {
			registrant, _, err := DefaultLedger.Store.GetRegistrant(pld.Name)
			if err != nil {
				return err
			}
			if len(registrant) == 0 {
				return fmt.Errorf("name doesn't exist")
			}
			if !bytes.Equal(registrant, pld.Registrant) {
				return fmt.Errorf("can not delete name which did not belongs to you")
			}
		}

	case pb.PayloadType_SUBSCRIBE_TYPE:
		pld := payload.(*pb.Subscribe)
		subscribed, err := DefaultLedger.Store.IsSubscribed(pld.Topic, pld.Bucket, pld.Subscriber, pld.Identifier)
		if err != nil {
			return err
		}
		if !subscribed && config.MaxSubscriptionsCount > 0 {
			subscriptionCount, err := DefaultLedger.Store.GetSubscribersCount(pld.Topic, pld.Bucket, nil, context.Background())
			if err != nil {
				return err
			}
			if subscriptionCount >= config.MaxSubscriptionsCount {
				return fmt.Errorf("subscription count to %s can't be more than %d", pld.Topic, config.MaxSubscriptionsCount)
			}
		}
	case pb.PayloadType_UNSUBSCRIBE_TYPE:
		pld := payload.(*pb.Unsubscribe)
		subscribed, err := DefaultLedger.Store.IsSubscribed(pld.Topic, 0, pld.Subscriber, pld.Identifier)
		if err != nil {
			return err
		}
		if !subscribed {
			return fmt.Errorf("subscription to %s doesn't exist", pld.Topic)
		}
	case pb.PayloadType_GENERATE_ID_TYPE:
		pld := payload.(*pb.GenerateID)
		id, idVersion, err := DefaultLedger.Store.GetIDVersion(pld.PublicKey)
		if err != nil {
			return err
		}
		if len(id) > 0 && pld.Version <= int32(idVersion) {
			return ErrIDRegistered
		}
		amount += pld.RegistrationFee
	case pb.PayloadType_NANO_PAY_TYPE:
		pld := payload.(*pb.NanoPay)

		channelBalance, _, err := DefaultLedger.Store.GetNanoPay(
			common.BytesToUint160(pld.Sender),
			common.BytesToUint160(pld.Recipient),
			pld.Id,
		)
		if err != nil {
			return err
		}

		if height > pld.TxnExpiration {
			return errors.New("nano pay txn has expired")
		}
		if height > pld.NanoPayExpiration {
			return errors.New("nano pay has expired")
		}

		balanceToClaim := pld.Amount - int64(channelBalance)
		if balanceToClaim <= 0 {
			return errors.New("invalid amount")
		}
		amount += balanceToClaim
	case pb.PayloadType_ISSUE_ASSET_TYPE:
		assetID := txn.Hash()
		_, _, _, _, err := DefaultLedger.Store.GetAsset(assetID)
		if err == nil {
			return ErrDuplicateIssueAssetTxn
		}
	default:
		return fmt.Errorf("invalid transaction payload type %v", txn.UnsignedTx.Payload.Type)
	}

	balance := DefaultLedger.Store.GetBalance(pg[0])
	if int64(balance) < amount+txn.UnsignedTx.Fee {
		return errors.New("not sufficient funds")
	}

	return nil
}

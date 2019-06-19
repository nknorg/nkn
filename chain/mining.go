package chain

import (
	"context"

	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nkn/vm/signature"
)

type Mining interface {
	BuildBlock(ctx context.Context, height uint32, chordID []byte, winnerHash common.Uint256, winnerType pb.WinnerType, timestamp int64) (*block.Block, error)
}

type BuiltinMining struct {
	account      *vault.Account // local account
	txnCollector *TxnCollector  // transaction pool
}

func NewBuiltinMining(account *vault.Account, txnCollector *TxnCollector) *BuiltinMining {
	return &BuiltinMining{
		account:      account,
		txnCollector: txnCollector,
	}
}

func (bm *BuiltinMining) BuildBlock(ctx context.Context, height uint32, chordID []byte, winnerHash common.Uint256, winnerType pb.WinnerType, timestamp int64) (*block.Block, error) {
	var txnList []*transaction.Transaction
	var txnHashList []common.Uint256

	donationAmount, err := DefaultLedger.Store.GetDonation()
	if err != nil {
		return nil, err
	}
	coinbase := bm.CreateCoinbaseTransaction(GetRewardByHeight(height) + donationAmount)
	txnList = append(txnList, coinbase)
	txnHashList = append(txnHashList, coinbase.Hash())
	totalTxsSize := coinbase.GetSize()
	txCount := 1

	if winnerType == pb.TXN_SIGNER {
		if _, err = DefaultLedger.Store.GetTransaction(winnerHash); err != nil {
			var miningSigChainTxn *transaction.Transaction
			miningSigChainTxn, err = por.GetPorServer().GetSigChainTxn(winnerHash)
			if err != nil {
				return nil, err
			}
			txnList = append(txnList, miningSigChainTxn)
			txnHashList = append(txnHashList, miningSigChainTxn.Hash())
			totalTxsSize = totalTxsSize + miningSigChainTxn.GetSize()
			txCount++
		}
	}

	txnCollection, err := bm.txnCollector.Collect()
	if err != nil {
		return nil, err
	}

	for {
		select {
		case <-ctx.Done():
			break
		default:
		}

		txn := txnCollection.Peek()
		if txn == nil {
			break
		}

		if txn.UnsignedTx.Fee < int64(config.Parameters.MinTxnFee) {
			log.Warning("transaction fee is too low")
			txnCollection.Pop()
			continue
		}

		totalTxsSize = totalTxsSize + txn.GetSize()
		if totalTxsSize > config.MaxBlockSize {
			break
		}

		txCount++
		if txCount > int(config.Parameters.NumTxnPerBlock) {
			break
		}

		txnHash := txn.Hash()
		if DefaultLedger.Store.IsTxHashDuplicate(txnHash) {
			log.Warning("it's a duplicate transaction")
			txnCollection.Pop()
			continue
		}

		txnList = append(txnList, txn)
		txnHashList = append(txnHashList, txnHash)
		if err = txnCollection.Update(); err != nil {
			txnCollection.Pop()
		}
	}

	txnRoot, err := crypto.ComputeRoot(txnHashList)
	if err != nil {
		return nil, err
	}
	encodedPubKey := bm.account.PublicKey.EncodePoint()
	randomBeacon := util.RandomBytes(config.RandomBeaconLength)
	curBlockHash := DefaultLedger.Store.GetCurrentBlockHash()
	header := &block.Header{
		Header: &pb.Header{
			UnsignedHeader: &pb.UnsignedHeader{
				Version:          HeaderVersion,
				PrevBlockHash:    curBlockHash.ToArray(),
				Timestamp:        timestamp,
				Height:           height,
				RandomBeacon:     randomBeacon,
				TransactionsRoot: txnRoot.ToArray(),
				WinnerHash:       winnerHash.ToArray(),
				WinnerType:       winnerType,
				Signer:           encodedPubKey,
				ChordId:          chordID,
			},
			Signature: nil,
		},
	}

	block := &block.Block{
		Header:       header,
		Transactions: txnList,
	}

	curStateHash, err := DefaultLedger.Store.GenerateStateRoot(block, true, false)
	if err != nil {
		return nil, err
	}

	header.UnsignedHeader.StateRoot = curStateHash.ToArray()

	hash := signature.GetHashForSigning(header)
	sig, err := crypto.Sign(bm.account.PrivateKey, hash)
	if err != nil {
		return nil, err
	}
	header.Signature = append(header.Signature, sig...)

	return block, nil
}

func (bm *BuiltinMining) CreateCoinbaseTransaction(reward common.Fixed64) *transaction.Transaction {
	// Transfer the reward to the beneficiary
	redeemHash := bm.account.ProgramHash
	if config.Parameters.BeneficiaryAddr != "" {
		hash, err := common.ToScriptHash(config.Parameters.BeneficiaryAddr)
		if err == nil {
			redeemHash = hash
		} else {
			log.Errorf("Convert beneficiary account to redeemhash error: %v", err)
		}
	}

	donationProgramhash, _ := common.ToScriptHash(config.DonationAddress)
	payload := transaction.NewCoinbase(donationProgramhash, redeemHash, reward)
	pl, err := transaction.Pack(pb.CoinbaseType, payload)
	if err != nil {
		return nil
	}

	nonce := DefaultLedger.Store.GetNonce(donationProgramhash)
	txn := transaction.NewMsgTx(pl, nonce, 0, util.RandomBytes(transaction.TransactionNonceLength))
	trans := &transaction.Transaction{
		Transaction: txn,
	}

	return trans
}

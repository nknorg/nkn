package chain

import (
	"context"

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
	"github.com/nknorg/nkn/vault"
)

type Mining interface {
	BuildBlock(ctx context.Context, height uint32, chordID []byte, winnerHash common.Uint256, winnerType pb.WinnerType) (*block.Block, error)
	SignBlock(b *block.Block, timestamp int64) error
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

func isBlockFull(txnCount uint32, txnSize uint32) bool {
	if config.Parameters.NumTxnPerBlock > 0 && txnCount > config.Parameters.NumTxnPerBlock {
		return true
	}
	if config.MaxBlockSize > 0 && txnSize > config.MaxBlockSize {
		return true
	}
	return false
}

func isLowFeeTxnFull(txnCount uint32, txnSize uint32) bool {
	if config.Parameters.NumLowFeeTxnPerBlock > 0 && txnCount > config.Parameters.NumLowFeeTxnPerBlock {
		return true
	}
	if config.Parameters.LowFeeTxnSizePerBlock > 0 && txnSize > config.Parameters.LowFeeTxnSizePerBlock {
		return true
	}
	return false
}

func (bm *BuiltinMining) BuildBlock(ctx context.Context, height uint32, chordID []byte, winnerHash common.Uint256, winnerType pb.WinnerType) (*block.Block, error) {
	var txnList []*transaction.Transaction
	var txnHashList []common.Uint256

	donationAmount, err := DefaultLedger.Store.GetDonation()
	if err != nil {
		return nil, err
	}
	coinbase := bm.CreateCoinbaseTransaction(GetRewardByHeight(height) + donationAmount)
	txnList = append(txnList, coinbase)
	txnHashList = append(txnHashList, coinbase.Hash())
	totalTxSize := coinbase.GetSize()
	totalTxCount := uint32(1)
	var lowFeeTxCount, lowFeeTxSize uint32

	if winnerType == pb.TXN_SIGNER {
		if _, err = DefaultLedger.Store.GetTransaction(winnerHash); err != nil {
			var miningSigChainTxn *transaction.Transaction
			miningSigChainTxn, err = por.GetPorServer().GetSigChainTxn(winnerHash)
			if err != nil {
				return nil, err
			}
			txnList = append(txnList, miningSigChainTxn)
			txnHashList = append(txnHashList, miningSigChainTxn.Hash())
			totalTxSize += miningSigChainTxn.GetSize()
			totalTxCount++
		}
	}

	txnCollection, err := bm.txnCollector.Collect()
	if err != nil {
		return nil, err
	}

	bvs := NewBlockValidationState()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		txn := txnCollection.Peek()
		if txn == nil {
			break
		}

		if isBlockFull(totalTxCount+1, totalTxSize+txn.GetSize()) {
			break
		}

		if txn.UnsignedTx.Fee < int64(config.Parameters.MinTxnFee) && isLowFeeTxnFull(lowFeeTxCount+1, lowFeeTxSize+txn.GetSize()) {
			log.Info("Low fee transaction full in block")
			break
		}

		if err := VerifyTransaction(txn); err != nil {
			log.Warningf("invalid transaction: %v", err)
			txnCollection.Pop()
			continue
		}
		if err := VerifyTransactionWithLedger(txn); err != nil {
			log.Warningf("invalid transaction: %v", err)
			txnCollection.Pop()
			continue
		}
		if err := bvs.VerifyTransactionWithBlock(txn, height); err != nil {
			log.Warningf("invalid transaction: %v", err)
			bvs.Reset()
			txnCollection.Pop()
			continue
		}

		txnList = append(txnList, txn)
		txnHashList = append(txnHashList, txn.Hash())
		if err = txnCollection.Update(); err != nil {
			bvs.Reset()
			txnCollection.Pop()
			continue
		}

		bvs.Commit()
		totalTxCount++
		totalTxSize += txn.GetSize()
		if txn.UnsignedTx.Fee < int64(config.Parameters.MinTxnFee) {
			lowFeeTxCount++
			lowFeeTxSize += txn.GetSize()
		}
	}

	bvs.Close()

	txnRoot, err := crypto.ComputeRoot(txnHashList)
	if err != nil {
		return nil, err
	}

	curBlockHash := DefaultLedger.Store.GetCurrentBlockHash()
	curHeader, err := DefaultLedger.Store.GetHeader(curBlockHash)
	if err != nil {
		return nil, err
	}

	curVrf := curHeader.UnsignedHeader.RandomBeacon[:config.RandomBeaconUniqueLength]
	vrf, proof, err := crypto.GenerateVrf(bm.account.PrivKey(), curVrf, true)
	if err != nil {
		return nil, err
	}
	randomBeacon := append(vrf, proof...)

	header := &block.Header{
		Header: &pb.Header{
			UnsignedHeader: &pb.UnsignedHeader{
				Version:          config.HeaderVersion,
				PrevBlockHash:    curBlockHash.ToArray(),
				Height:           height,
				RandomBeacon:     randomBeacon,
				TransactionsRoot: txnRoot.ToArray(),
				WinnerHash:       winnerHash.ToArray(),
				WinnerType:       winnerType,
				SignerPk:         bm.account.PublicKey.EncodePoint(),
				SignerId:         chordID,
			},
			Signature: nil,
		},
	}

	block := &block.Block{
		Header:       header,
		Transactions: txnList,
	}

	curStateHash, err := DefaultLedger.Store.GenerateStateRoot(ctx, block, true, false)
	if err != nil {
		return nil, err
	}

	header.UnsignedHeader.StateRoot = curStateHash.ToArray()

	return block, nil
}

func (bm *BuiltinMining) SignBlock(b *block.Block, timestamp int64) error {
	b.Header.UnsignedHeader.Timestamp = timestamp

	hash := signature.GetHashForSigning(b.Header)
	sig, err := crypto.Sign(bm.account.PrivateKey, hash)
	if err != nil {
		return err
	}
	b.Header.Signature = append(b.Header.Signature, sig...)
	return nil
}

func (bm *BuiltinMining) CreateCoinbaseTransaction(reward common.Fixed64) *transaction.Transaction {
	// Transfer the reward to the beneficiary
	redeemHash := bm.account.ProgramHash
	if len(config.Parameters.BeneficiaryAddr) > 0 {
		hash, err := common.ToScriptHash(config.Parameters.BeneficiaryAddr)
		if err == nil {
			redeemHash = hash
		} else {
			log.Errorf("Convert beneficiary account to redeemhash error: %v", err)
		}
	}

	donationProgramhash, _ := common.ToScriptHash(config.DonationAddress)
	payload := transaction.NewCoinbase(donationProgramhash, redeemHash, reward)
	pl, err := transaction.Pack(pb.COINBASE_TYPE, payload)
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

package ledger

import (
	"errors"

	tx "github.com/nknorg/nkn/core/transaction"
	. "github.com/nknorg/nkn/errors"
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
	if header.Height == 0 {
		return nil
	}
	prevHeader, err := ledger.Blockchain.GetHeader(header.PrevBlockHash)
	if err != nil {
		return errors.New("prev header doesn't exist")
	}
	if prevHeader == nil {
		return errors.New("invalid prev header")
	}
	if prevHeader.Height+1 != header.Height {
		return errors.New("invalid header height")
	}
	if prevHeader.Timestamp >= header.Timestamp {
		return errors.New("invalid header timestamp")
	}

	return nil
}

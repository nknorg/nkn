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
	"github.com/nknorg/nkn/util/log"
)

const (
	GenesisBlockProposedHeight = 4
)

type VBlock struct {
	Block       *Block
	ReceiveTime int64
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

	return nil
}

func HeaderCheck(header *Header, receiveTime int64) error {
	height := header.Height
	if height == 0 {
		return nil
	}
	prevHeader, err := DefaultLedger.Blockchain.GetHeader(header.PrevBlockHash)
	if err != nil {
		return errors.New("prev header doesn't exist")
	}
	if prevHeader == nil {
		return errors.New("invalid prev header")
	}
	if prevHeader.Height+1 != height {
		return errors.New("invalid header height")
	}
	if prevHeader.Timestamp >= header.Timestamp {
		return errors.New("invalid header timestamp")
	}
	if header.WinningHashType == GenesisHash && header.Height >= GenesisBlockProposedHeight {
		return errors.New("invalid winning hash type")
	}

	// calculate time difference
	var timeDiff int64
	genesisBlockHash, err := DefaultLedger.Store.GetBlockHash(0)
	if err != nil {
		return err
	}
	genesisBlock, err := DefaultLedger.Store.GetBlock(genesisBlockHash)
	if err != nil {
		return err
	}
	prevTimestamp, err := DefaultLedger.Blockchain.GetBlockTime(header.PrevBlockHash)
	if err != nil {
		return err
	}
	if prevTimestamp == genesisBlock.Header.Timestamp {
		timeDiff = 0
	} else {
		timeDiff = receiveTime - prevTimestamp
	}

	// get miner who will sign next block
	var publicKey []byte
	var chordID []byte
	timeSlot := int64(config.ProposerChangeTime / time.Second)
	if timeDiff >= timeSlot {
		// This is a temporary solution
		proposerBlockHeight := 0
		// index := timeDiff / timeSlot
		// proposerBlockHeight := int64(DefaultLedger.Store.GetHeight()) - index
		// if proposerBlockHeight < 0 {
		// proposerBlockHeight = 0
		// }
		proposerBlockHash, err := DefaultLedger.Store.GetBlockHash(uint32(proposerBlockHeight))
		if err != nil {
			return err
		}
		proposerBlock, err := DefaultLedger.Store.GetBlock(proposerBlockHash)
		if err != nil {
			return err
		}
		publicKey, chordID, err = proposerBlock.GetSigner()
		log.Infof("block signer: public key should be %s, chord ID should be %s, "+
			"which is the signer of block %d", BytesToHexString(publicKey),
			BytesToHexString(chordID), proposerBlockHeight)
		if err != nil {
			return err
		}
	} else {
		winningHash := prevHeader.WinningHash
		winningHashType := prevHeader.WinningHashType
		switch winningHashType {
		case GenesisHash:
			publicKey, chordID, err = genesisBlock.GetSigner()
			if err != nil {
				return err
			}
			log.Infof("block signer: public key should be %s, which is genesis block proposer",
				BytesToHexString(publicKey))
		case WinningTxnHash:
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
			publicKey, chordID, err = sigchain.GetMiner()
			if err != nil {
				return err
			}
			txnHash := txn.Hash()
			log.Infof("block signer: public key should be %s, chord ID should be %s, "+
				"which is got in sigchain transaction %s", BytesToHexString(publicKey), BytesToHexString(chordID),
				BytesToHexString(txnHash.ToArrayReverse()))
		}
	}
	// TODO check chord ID is valid
	_ = chordID
	// verify if public is expected
	if bytes.Compare(publicKey, header.Signer) != 0 {
		return fmt.Errorf("invalid block signer public key: %s", BytesToHexString(header.Signer))
	}
	rawPubKey, err := crypto.DecodePoint(publicKey)
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

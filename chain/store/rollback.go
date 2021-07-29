package store

import (
	"bytes"
	"errors"

	"github.com/nknorg/nkn/v2/block"
	"github.com/nknorg/nkn/v2/chain/db"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/common/serialization"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/util/log"
)

func (cs *ChainStore) Rollback(b *block.Block) error {
	log.Warning("Start rollback.")

	if err := cs.st.NewBatch(); err != nil {
		return err
	}

	if b.Header.UnsignedHeader.Height == 0 {
		return errors.New("the genesis block need not be rolled back")
	}

	if err := cs.rollbackHeader(b); err != nil {
		return err
	}

	if err := cs.rollbackTransaction(b); err != nil {
		return err
	}

	if err := cs.rollbackBlockHash(b); err != nil {
		return err
	}

	if err := cs.rollbackCurrentBlockHash(b); err != nil {
		return err
	}

	if err := cs.rollbackDonation(b); err != nil {
		return err
	}

	if err := cs.rollbackStates(b); err != nil {
		return err
	}

	if err := cs.rollbackHeaderCache(b); err != nil {
		return err
	}

	if err := cs.rollbackSigChainCache(b); err != nil {
		return err
	}

	if err := cs.rollbackRefCounts(b); err != nil {
		return err
	}

	if err := cs.st.BatchCommit(); err != nil {
		return err
	}

	prevHash, err := common.Uint256ParseFromBytes(b.Header.UnsignedHeader.PrevBlockHash)
	if err != nil {
		return err
	}

	cs.mu.Lock()
	cs.currentBlockHeight = b.Header.UnsignedHeader.Height - 1
	cs.currentBlockHash = prevHash
	cs.mu.Unlock()

	return nil
}

func (cs *ChainStore) rollbackHeader(b *block.Block) error {
	blockHash := b.Hash()
	return cs.st.BatchDelete(db.HeaderKey(blockHash))
}

func (cs *ChainStore) rollbackTransaction(b *block.Block) error {
	for _, txn := range b.Transactions {
		txHash := txn.Hash()
		if err := cs.st.BatchDelete(db.TransactionKey(txHash)); err != nil {
			return err
		}
	}

	return nil
}

func (cs *ChainStore) rollbackBlockHash(b *block.Block) error {
	return cs.st.BatchDelete(db.BlockhashKey(b.Header.UnsignedHeader.Height))
}

func (cs *ChainStore) rollbackCurrentBlockHash(b *block.Block) error {
	value := new(bytes.Buffer)
	prevHash, err := common.Uint256ParseFromBytes(b.Header.UnsignedHeader.PrevBlockHash)
	if err != nil {
		return err
	}
	if _, err := prevHash.Serialize(value); err != nil {
		return err
	}
	if err := serialization.WriteUint32(value, b.Header.UnsignedHeader.Height-1); err != nil {
		return err
	}

	return cs.st.BatchPut(db.CurrentBlockHashKey(), value.Bytes())
}

func (cs *ChainStore) rollbackStates(b *block.Block) error {
	prevHash, err := common.Uint256ParseFromBytes(b.Header.UnsignedHeader.PrevBlockHash)
	if err != nil {
		return err
	}

	prevHead, err := cs.GetHeader(prevHash)
	if err != nil {
		return err
	}

	root, err := common.Uint256ParseFromBytes(prevHead.UnsignedHeader.StateRoot)
	if err != nil {
		return err
	}

	cs.States, err = NewStateDB(root, cs)
	if err != nil {
		return err
	}

	err = cs.st.BatchPut(db.CurrentStateTrie(), root.ToArray())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) rollbackHeaderCache(b *block.Block) error {
	cs.headerCache.RollbackHeader(b.Header)
	return nil
}

func (cs *ChainStore) rollbackSigChainCache(b *block.Block) error {
	cs.sigChainCache.RollbackSigChain(b.Header)
	return nil
}

func (cs *ChainStore) rollbackDonation(b *block.Block) error {
	if b.Header.UnsignedHeader.Height%uint32(config.RewardAdjustInterval) != 0 || config.DonationNoDelay.GetValueAtHeight(b.Header.UnsignedHeader.Height) {
		return nil
	}

	cs.st.BatchDelete(db.DonationKey(b.Header.UnsignedHeader.Height))
	return nil
}

func (cs *ChainStore) rollbackRefCounts(b *block.Block) error {
	states, err := NewStateDB(common.EmptyUint256, cs)
	if err != nil {
		return err
	}

	return states.RollbackPruning(b)
}

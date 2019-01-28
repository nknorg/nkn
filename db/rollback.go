package db

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/types"
)

func (cs *ChainStore) Rollback(b *types.Block) error {
	if err := cs.st.NewBatch(); err != nil {
		return err
	}

	if b.Header.UnsignedHeader.Height == 0 {
		return errors.New("the genesis block need not be rolled back.")
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

	if err := cs.rollbackNames(b); err != nil {
		return err
	}

	if err := cs.rollbackPubSub(b); err != nil {
		return err
	}

	if err := cs.st.BatchCommit(); err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) rollbackHeader(b *types.Block) error {
	blockHash := b.Hash()
	return cs.st.BatchDelete(append([]byte{byte(DATA_Header)}, blockHash[:]...))
}

func (cs *ChainStore) rollbackTransaction(b *types.Block) error {
	for _, txn := range b.Transactions {
		txHash := txn.Hash()
		if err := cs.st.BatchDelete(append([]byte{byte(DATA_Transaction)}, txHash[:]...)); err != nil {
			return err
		}
	}

	return nil
}

func (cs *ChainStore) rollbackBlockHash(b *types.Block) error {
	height := make([]byte, 4)
	binary.LittleEndian.PutUint32(height[:], b.Header.UnsignedHeader.Height)
	return cs.st.BatchDelete(append([]byte{byte(DATA_BlockHash)}, height...))
}

func (cs *ChainStore) rollbackCurrentBlockHash(b *types.Block) error {
	value := new(bytes.Buffer)
	prevHash, _ := common.Uint256ParseFromBytes(b.Header.UnsignedHeader.PrevBlockHash)
	if _, err := prevHash.Serialize(value); err != nil {
		return err
	}
	if err := serialization.WriteUint32(value, b.Header.UnsignedHeader.Height-1); err != nil {
		return err
	}

	return cs.st.BatchPut([]byte{byte(SYS_CurrentBlock)}, value.Bytes())
}

func (cs *ChainStore) rollbackNames(b *types.Block) error {
	for _, txn := range b.Transactions {
		if txn.UnsignedTx.Payload.Type == types.RegisterNameType {
			pl, err := types.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				return err
			}

			registerNamePayload := pl.(*types.RegisterName)
			err = cs.DeleteName(registerNamePayload.Registrant)
			if err != nil {
				return err
			}
		}
	}

	for _, txn := range b.Transactions {
		if txn.UnsignedTx.Payload.Type == types.DeleteNameType {
			pl, err := types.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				return err
			}

			deleteNamePayload := pl.(*types.DeleteName)
			err = cs.SaveName(deleteNamePayload.Registrant, deleteNamePayload.Name)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (cs *ChainStore) rollbackPubSub(b *types.Block) error {
	height := b.Header.UnsignedHeader.Height

	for _, txn := range b.Transactions {
		if txn.UnsignedTx.Payload.Type == types.SubscribeType {
			pl, err := types.Unpack(txn.UnsignedTx.Payload)
			if err != nil {
				return err
			}

			subscribePayload := pl.(*types.Subscribe)
			err = cs.Unsubscribe(subscribePayload.Subscriber, subscribePayload.Identifier, subscribePayload.Topic, subscribePayload.Bucket, subscribePayload.Duration, height)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

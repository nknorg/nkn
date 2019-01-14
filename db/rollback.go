package db

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/ledger"
	tx "github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
)

func (cs *ChainStore) Rollback(b *ledger.Block) error {
	if err := cs.st.NewBatch(); err != nil {
		return err
	}

	if b.Header.Height == 0 {
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

	if err := cs.rollbackAsset(b); err != nil {
		return err
	}

	if err := cs.rollbackPubSub(b); err != nil {
		return err
	}

	if err := cs.rollbackUnspentIndex(b); err != nil {
		return err
	}

	if err := cs.rollbackUTXO(b); err != nil {
		return err
	}

	if err := cs.rollbackPrepaidAndWithdraw(b); err != nil {
		return err
	}

	if err := cs.rollbackIssued(b); err != nil {
		return err
	}

	if err := cs.rollbackHeaderHashlist(b); err != nil {
		return err
	}

	if err := cs.st.BatchCommit(); err != nil {
		return err
	}

	return cs.rollbackCached(b)
}

func (cs *ChainStore) rollbackHeader(b *ledger.Block) error {
	blockHash := b.Hash()
	return cs.st.BatchDelete(append([]byte{byte(DATA_Header)}, blockHash[:]...))
}

func (cs *ChainStore) rollbackTransaction(b *ledger.Block) error {
	for _, txn := range b.Transactions {
		txHash := txn.Hash()
		if err := cs.st.BatchDelete(append([]byte{byte(DATA_Transaction)}, txHash[:]...)); err != nil {
			return err
		}
	}

	return nil
}

func (cs *ChainStore) rollbackBlockHash(b *ledger.Block) error {
	height := make([]byte, 4)
	binary.LittleEndian.PutUint32(height[:], b.Header.Height)
	return cs.st.BatchDelete(append([]byte{byte(DATA_BlockHash)}, height...))
}

func (cs *ChainStore) rollbackCurrentBlockHash(b *ledger.Block) error {
	value := new(bytes.Buffer)
	if _, err := b.Header.PrevBlockHash.Serialize(value); err != nil {
		return err
	}
	if err := serialization.WriteUint32(value, b.Header.Height-1); err != nil {
		return err
	}

	return cs.st.BatchPut([]byte{byte(SYS_CurrentBlock)}, value.Bytes())
}

func (cs *ChainStore) rollbackHeaderHashlist(b *ledger.Block) error {
	hash := b.Hash()
	iter := cs.st.NewIterator([]byte{byte(IX_HeaderHashList)})
	for iter.Next() {
		r := bytes.NewReader(iter.Value())
		storedHeaderCount, err := serialization.ReadVarUint(r, 0)
		if err != nil {
			return err
		}

		headerIndex := make([]common.Uint256, 0)
		for i := 0; i < int(storedHeaderCount); i++ {
			var listHash common.Uint256
			listHash.Deserialize(r)
			headerIndex = append(headerIndex, listHash)
		}

		if hash.CompareTo(headerIndex[len(headerIndex)-1]) == 0 {
			if err := cs.st.BatchDelete(iter.Key()); err != nil {
				return err
			}
			break
		}
	}
	iter.Release()

	return nil

}

func (cs *ChainStore) rollbackUnspentIndex(b *ledger.Block) error {
	unspents := make(map[common.Uint256][]uint16)
	for _, txn := range b.Transactions {
		txhash := txn.Hash()
		cs.st.BatchDelete(append([]byte{byte(IX_Unspent)}, txhash.ToArray()...))

		for _, input := range txn.Inputs {
			referTxnHash := input.ReferTxID
			referTxnOutIndex := input.ReferTxOutputIndex
			if _, ok := unspents[referTxnHash]; !ok {
				if unspentValue, err := cs.st.Get(append([]byte{byte(IX_Unspent)}, referTxnHash.ToArray()...)); err != nil {
					unspents[referTxnHash] = []uint16{}
				} else {
					if unspents[referTxnHash], err = common.GetUint16Array(unspentValue); err != nil {
						return err
					}
				}
			}
			unspents[referTxnHash] = append(unspents[referTxnHash], referTxnOutIndex)
		}
	}

	for txhash, value := range unspents {
		cs.st.BatchPut(append([]byte{byte(IX_Unspent)}, txhash.ToArray()...), common.ToByteArray(value))
	}

	return nil
}

func (cs *ChainStore) rollbackUTXO(b *ledger.Block) error {
	unspendUTXOs := make(map[common.Uint160]map[common.Uint256]map[uint32][]*tx.UTXOUnspent)
	height := b.Header.Height

	for _, txn := range b.Transactions {
		for _, output := range txn.Outputs {
			heightBuffer := make([]byte, 4)
			binary.LittleEndian.PutUint32(heightBuffer[:], height)
			key := append(append(output.ProgramHash.ToArray(), output.AssetID.ToArray()...), heightBuffer...)
			cs.st.BatchDelete(append([]byte{byte(IX_Unspent_UTXO)}, key...))
		}

		for _, input := range txn.Inputs {
			referTxn, hh, err := cs.getTx(input.ReferTxID)
			if err != nil {
				return err
			}

			index := input.ReferTxOutputIndex
			referTxnOutput := referTxn.Outputs[index]
			programHash := referTxnOutput.ProgramHash
			assetID := referTxnOutput.AssetID

			if _, ok := unspendUTXOs[programHash]; !ok {
				unspendUTXOs[programHash] = make(map[common.Uint256]map[uint32][]*tx.UTXOUnspent)
			}
			if _, ok := unspendUTXOs[programHash][assetID]; !ok {
				unspendUTXOs[programHash][assetID] = make(map[uint32][]*tx.UTXOUnspent)
			}
			if _, ok := unspendUTXOs[programHash][assetID][hh]; !ok {
				if unspendUTXOs[programHash][assetID][hh], err = cs.GetUnspentByHeight(programHash, assetID, hh); err != nil {
					unspendUTXOs[programHash][assetID][hh] = make([]*tx.UTXOUnspent, 0)
				}
			}

			u := tx.UTXOUnspent{
				Txid:  referTxn.Hash(),
				Index: uint32(index),
				Value: referTxnOutput.Value,
			}
			unspendUTXOs[programHash][assetID][hh] = append(unspendUTXOs[programHash][assetID][hh], &u)
		}

	}

	for programHash, programHash_value := range unspendUTXOs {
		for assetId, unspents := range programHash_value {
			for height, unspent := range unspents {
				heightBuffer := make([]byte, 4)
				binary.LittleEndian.PutUint32(heightBuffer[:], height)
				key := append(append(programHash.ToArray(), assetId.ToArray()...), heightBuffer...)

				//TODO if the listnum is 0?
				listnum := len(unspent)
				if listnum == 0 {
					if err := cs.st.BatchDelete(key); err != nil {
						return err
					}
					continue
				}

				w := bytes.NewBuffer(nil)
				serialization.WriteVarUint(w, uint64(listnum))
				for i := 0; i < listnum; i++ {
					unspent[i].Serialize(w)
				}

				if err := cs.st.BatchPut(key, w.Bytes()); err != nil {
					return err
				}
			}

		}
	}

	return nil
}

func (cs *ChainStore) rollbackAsset(b *ledger.Block) error {
	for _, txn := range b.Transactions {
		if txn.TxType == tx.RegisterAsset {
			txhash := txn.Hash()
			if err := cs.st.BatchDelete(append([]byte{byte(ST_Info)}, txhash.ToArray()...)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (cs *ChainStore) rollbackPubSub(b *ledger.Block) error {
	height := b.Header.Height

	for _, txn := range b.Transactions {
		if txn.TxType == tx.Subscribe {
			subscribePayload := txn.Payload.(*payload.Subscribe)
			err := cs.Unsubscribe(subscribePayload.Subscriber, subscribePayload.Identifier, subscribePayload.Topic, subscribePayload.Bucket, subscribePayload.Duration, height)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (cs *ChainStore) rollbackIssued(b *ledger.Block) error {
	quantities := make(map[common.Uint256]common.Fixed64)

	for _, txn := range b.Transactions {
		if txn.TxType != tx.IssueAsset {
			continue
		}

		results := txn.GetMergedAssetIDValueFromOutputs()
		for assetId, value := range results {
			if _, ok := quantities[assetId]; !ok {
				quantities[assetId] += value
			} else {
				quantities[assetId] = value
			}
		}
	}

	for assetId, value := range quantities {
		data, err := cs.st.Get(append([]byte{byte(ST_QuantityIssued)}, assetId.ToArray()...))
		if err != nil {
			return err
		}

		var qt common.Fixed64
		if err := qt.Deserialize(bytes.NewReader(data)); err != nil {
			return err
		}

		qt = qt - value
		quantity := bytes.NewBuffer(nil)
		if err := qt.Serialize(quantity); err != nil {
			return err
		}

		if qt == common.Fixed64(0) {
			if err := cs.st.BatchDelete(append([]byte{byte(ST_QuantityIssued)}, assetId.ToArray()...)); err != nil {
				return err
			}
		} else {
			if err := cs.st.BatchPut(append([]byte{byte(ST_QuantityIssued)}, assetId.ToArray()...), quantity.Bytes()); err != nil {
				return err
			}
		}
	}

	return nil
}

func (cs *ChainStore) rollbackPrepaidAndWithdraw(b *ledger.Block) error {
	type prepaid struct {
		amount common.Fixed64
		rates  common.Fixed64
	}
	deposits := make(map[common.Uint160]prepaid)

	for _, txn := range b.Transactions {
		if txn.TxType != tx.Withdraw {
			continue
		}

		withdrawPld, ok := txn.Payload.(*payload.Withdraw)
		if !ok {
			return errors.New("transaction type error")
		}

		if _, ok := deposits[withdrawPld.ProgramHash]; !ok {
			if amount, rates, err := cs.GetPrepaidInfo(withdrawPld.ProgramHash); err != nil {
				return err
			} else {
				deposits[withdrawPld.ProgramHash] = prepaid{amount: *amount, rates: *rates}
			}
		}

		newAmount := deposits[withdrawPld.ProgramHash].amount + txn.Outputs[0].Value
		deposits[withdrawPld.ProgramHash] = prepaid{amount: newAmount, rates: deposits[withdrawPld.ProgramHash].rates}
	}

	for _, txn := range b.Transactions {
		if txn.TxType != tx.Prepaid {
			continue
		}

		prepaidPld, ok := txn.Payload.(*payload.Prepaid)
		if !ok {
			return errors.New("this is not Prepaid transaciton")
		}

		pHash, err := txn.GetProgramHashes()
		if err != nil || len(pHash) == 0 {
			return errors.New("no programhash")
		}

		if _, ok := deposits[pHash[0]]; !ok {
			if amount, rates, err := cs.GetPrepaidInfo(pHash[0]); err != nil {
				return err
			} else {
				deposits[pHash[0]] = prepaid{amount: *amount, rates: *rates}
			}
		}

		newAmount := deposits[pHash[0]].amount - prepaidPld.Amount
		deposits[pHash[0]] = prepaid{amount: newAmount, rates: deposits[pHash[0]].rates}
	}

	for programhash, deposit := range deposits {
		if deposit.amount == common.Fixed64(0) {
			cs.st.BatchDelete(append([]byte{byte(ST_Prepaid)}, programhash.ToArray()...))
			continue
		}

		value := bytes.NewBuffer(nil)
		if err := deposit.amount.Serialize(value); err != nil {
			return err
		}

		if err := deposit.rates.Serialize(value); err != nil {
			return err
		}

		if err := cs.st.BatchPut(append([]byte{byte(ST_Prepaid)}, programhash.ToArray()...), value.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

func (cs *ChainStore) rollbackCached(b *ledger.Block) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	hash := b.Hash()
	if h, ok := cs.headerIndex[b.Header.Height]; ok {
		if h.CompareTo(hash) == 0 {
			delete(cs.headerIndex, b.Header.Height)
		}
	}

	if _, ok := cs.blockCache[hash]; ok {
		delete(cs.blockCache, hash)
	}

	if _, ok := cs.headerCache[hash]; ok {
		delete(cs.headerCache, hash)
	}

	return nil
}

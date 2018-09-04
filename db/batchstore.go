package db

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
)

type BatchStore struct {
	store *ChainStore
	batch IBatch
}

func NewBatchStore(store *ChainStore) *BatchStore {
	return &BatchStore{
		store: store,
		batch: store.st.NewSepBatch(),
	}

}

func (bs *BatchStore) Commit() error {
	return bs.batch.BatchCommit()
}

func (bs *BatchStore) storeBlock(b *ledger.Block) error {
	headerHash := b.Hash()

	//batch put header
	headerBuffer := bytes.NewBuffer(nil)
	b.Trim(headerBuffer)
	if err := bs.batch.BatchPut(headerKey(headerHash), headerBuffer.Bytes()); err != nil {
		return err
	}

	//batch put headerhash
	headerHashBuffer := bytes.NewBuffer(nil)
	headerHash.Serialize(headerHashBuffer)
	if err := bs.batch.BatchPut(blockhashKey(b.Header.Height), headerHashBuffer.Bytes()); err != nil {
		return err
	}

	//batch put transactions
	for _, tx := range b.Transactions {
		w := bytes.NewBuffer(nil)
		serialization.WriteUint32(w, b.Header.Height)
		tx.Serialize(w)

		if err := bs.batch.BatchPut(transactionKey(tx.Hash()), w.Bytes()); err != nil {
			return err
		}
	}

	//batch put currentblockhash
	serialization.WriteUint32(headerHashBuffer, b.Header.Height)
	err := bs.batch.BatchPut(currentBlockHashKey(), headerHashBuffer.Bytes())

	return err
}

func (bs *BatchStore) storeSpcialTxs(txs []*transaction.Transaction) error {
	var regAssetTxs, issAssetTxs, prepaidTxs, withdrawTxs, bkperTxs []*transaction.Transaction
	for _, txn := range txs {
		switch txn.TxType {
		case transaction.RegisterAsset:
			regAssetTxs = append(regAssetTxs, txn)
		case transaction.IssueAsset:
			issAssetTxs = append(issAssetTxs, txn)
		case transaction.Prepaid:
			prepaidTxs = append(prepaidTxs, txn)
		case transaction.Withdraw:
			withdrawTxs = append(withdrawTxs, txn)
		case transaction.BookKeeper:
			bkperTxs = append(bkperTxs, txn)
		}
	}

	if bs.storeAssets(regAssetTxs) != nil ||
		bs.storeQuantityIssued(issAssetTxs) != nil ||
		bs.storePrepaid(prepaidTxs) != nil ||
		bs.storeWithdraw(withdrawTxs) != nil ||
		bs.storeBookkeepers(bkperTxs) != nil {
		return errors.New("persist special tx error")
	}

	return nil
}

func (bs *BatchStore) storeAssets(txs []*transaction.Transaction) error {
	for _, tx := range txs {
		if tx.TxType != transaction.RegisterAsset {
			continue
		}

		ar := tx.Payload.(*payload.RegisterAsset) //TODO error handle
		w := bytes.NewBuffer(nil)
		ar.Asset.Serialize(w) //TODO error handle

		err := bs.batch.BatchPut(assetKey(tx.Hash()), w.Bytes())
		if err != nil {
			return err
		}
	}

	return nil
}

func (bs *BatchStore) storeQuantityIssued(txs []*transaction.Transaction) error {
	quantities := make(map[common.Uint256]common.Fixed64)
	for _, tx := range txs {
		if tx.TxType != transaction.IssueAsset {
			continue
		}

		results := tx.GetMergedAssetIDValueFromOutputs()
		for assetId, value := range results {
			if _, ok := quantities[assetId]; !ok {
				quantities[assetId] += value
			} else {
				quantities[assetId] = value
			}
		}
	}

	for assetId, value := range quantities {
		qt, err := bs.store.GetQuantityIssued(assetId)
		if err != nil {
			return err
		}

		qt = qt + value
		quantity := bytes.NewBuffer(nil)
		qt.Serialize(quantity)

		if err := bs.batch.BatchPut(issuseQuantKey(assetId), quantity.Bytes()); err != nil {
			return err
		}

	}

	return nil
}

func (bs *BatchStore) storePrepaid(txs []*transaction.Transaction) error {
	for _, tx := range txs {
		if tx.TxType != transaction.Prepaid {
			continue
		}

		prepaidPld := tx.Payload.(*payload.Prepaid)
		pHash, err := tx.GetProgramHashes() //TODO tx.ProgramHash ??
		if err != nil || len(pHash) == 0 {
			return errors.New("no programhash")
		}

		newAmount := prepaidPld.Amount
		value := bytes.NewBuffer(nil)
		oldAmount, _, _ := bs.store.GetPrepaidInfo(pHash[0])
		if oldAmount != nil {
			newAmount = *oldAmount + newAmount
		}
		if err := newAmount.Serialize(value); err != nil {
			return err
		}

		if err := prepaidPld.Rates.Serialize(value); err != nil {
			return err
		}

		if err := bs.batch.BatchPut(prepaidKey(pHash[0]), value.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

func (bs *BatchStore) storeWithdraw(txs []*transaction.Transaction) error {
	for _, tx := range txs {
		if tx.TxType != transaction.Withdraw {
			continue
		}

		withdrawPld := tx.Payload.(*payload.Withdraw)
		// TODO for range output list

		newAmount := tx.Outputs[0].Value
		value := bytes.NewBuffer(nil)
		oldAmount, rates, _ := bs.store.GetPrepaidInfo(withdrawPld.ProgramHash)
		if oldAmount != nil {
			newAmount = *oldAmount - newAmount
		}
		if err := newAmount.Serialize(value); err != nil {
			return err
		}
		//TODO rates is nil ?
		if err := rates.Serialize(value); err != nil {
			return err
		}

		if err := bs.batch.BatchPut(prepaidKey(withdrawPld.ProgramHash), value.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

func (bs *BatchStore) storeBookkeepers(txs []*transaction.Transaction) error {
	needUpdateBookKeeper := false
	currBookKeeper, nextBookKeeper, err := bs.store.GetBookKeeperList()
	if err != nil {
		return err
	}

	if len(currBookKeeper) != len(nextBookKeeper) {
		needUpdateBookKeeper = true
	} else {
		for i := range currBookKeeper {
			if currBookKeeper[i].X.Cmp(nextBookKeeper[i].X) != 0 ||
				currBookKeeper[i].Y.Cmp(nextBookKeeper[i].Y) != 0 {
				needUpdateBookKeeper = true
				break
			}
		}
	}

	if needUpdateBookKeeper {
		currBookKeeper = make([]*crypto.PubKey, len(nextBookKeeper))
		for i := 0; i < len(nextBookKeeper); i++ {
			currBookKeeper[i] = new(crypto.PubKey)
			currBookKeeper[i].X = new(big.Int).Set(nextBookKeeper[i].X)
			currBookKeeper[i].Y = new(big.Int).Set(nextBookKeeper[i].Y)
		}
	}

	for _, tx := range txs {
		if tx.TxType != transaction.BookKeeper {
			continue
		}

		bk := tx.Payload.(*payload.BookKeeper)
		switch bk.Action {
		case payload.BookKeeperAction_ADD:
			findflag := false
			for k := 0; k < len(nextBookKeeper); k++ {
				if bk.PubKey.X.Cmp(nextBookKeeper[k].X) == 0 && bk.PubKey.Y.Cmp(nextBookKeeper[k].Y) == 0 {
					findflag = true
					break
				}
			}

			if !findflag {
				needUpdateBookKeeper = true
				nextBookKeeper = append(nextBookKeeper, bk.PubKey)
				sort.Sort(crypto.PubKeySlice(nextBookKeeper))
			}
		case payload.BookKeeperAction_SUB:
			ind := -1
			for k := 0; k < len(nextBookKeeper); k++ {
				if bk.PubKey.X.Cmp(nextBookKeeper[k].X) == 0 && bk.PubKey.Y.Cmp(nextBookKeeper[k].Y) == 0 {
					ind = k
					break
				}
			}

			if ind != -1 {
				needUpdateBookKeeper = true
				nextBookKeeper = append(nextBookKeeper[:ind], nextBookKeeper[ind+1:]...)
			}
		}

	}

	if needUpdateBookKeeper {
		bkList := bytes.NewBuffer(nil)

		serialization.WriteUint8(bkList, uint8(len(currBookKeeper)))
		for k := 0; k < len(currBookKeeper); k++ {
			currBookKeeper[k].Serialize(bkList)
		}

		serialization.WriteUint8(bkList, uint8(len(nextBookKeeper)))
		for k := 0; k < len(nextBookKeeper); k++ {
			nextBookKeeper[k].Serialize(bkList)
		}

		bs.batch.BatchPut(bookkeeperKey(), bkList.Bytes())
	}

	return nil
}

func (bs *BatchStore) storeUnspentIndexs(txs []*transaction.Transaction) error {
	unspents := make(map[common.Uint256][]uint16)

	for _, tx := range txs {
		txhash := tx.Hash()

		// delete unspent index when spent in input
		for _, input := range tx.Inputs {
			spendTxhash := input.ReferTxID
			if _, ok := unspents[spendTxhash]; !ok {
				spend, err := bs.store.getUnspentIndex(spendTxhash)
				if err != nil {
					return err
				}

				unspents[spendTxhash] = spend
			}

			unspentLen := len(unspents[txhash])
			for i, outputIndex := range unspents[spendTxhash] {
				if outputIndex == uint16(input.ReferTxOutputIndex) {
					unspents[txhash][i] = unspents[txhash][unspentLen-1]
					unspents[txhash] = unspents[txhash][:unspentLen-1]
					break
				}
			}
		}

		for i, _ := range tx.Outputs {
			unspents[txhash] = append(unspents[txhash], uint16(i))
		}
	}

	for txhash, value := range unspents {
		if len(value) == 0 {
			if err := bs.batch.BatchDelete(unspentIndexKey(txhash)); err != nil {
				return err
			}
		} else {
			if err := bs.batch.BatchPut(unspentIndexKey(txhash), common.ToByteArray(value)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (bs *BatchStore) storeUTXOs(height uint32, txs []*transaction.Transaction) error {
	unspents := make(map[common.Uint160]map[common.Uint256]map[uint32][]*transaction.UTXOUnspent)

	for _, tx := range txs {
		for _, input := range tx.Inputs {
			spentTx, height, err := bs.store.getTx(input.ReferTxID) //TODO delete getTx
			if err != nil {
				return err
			}
			programHash := spentTx.Outputs[input.ReferTxOutputIndex].ProgramHash
			assetId := spentTx.Outputs[input.ReferTxOutputIndex].AssetID

			if _, ok := unspents[programHash]; !ok {
				unspents[programHash] = make(map[common.Uint256]map[uint32][]*transaction.UTXOUnspent)
			}

			if _, ok := unspents[programHash][assetId]; !ok {
				unspents[programHash][assetId] = make(map[uint32][]*transaction.UTXOUnspent)
			}

			if _, ok := unspents[programHash][assetId][height]; !ok {
				var err error
				unspents[programHash][assetId][height], err = bs.store.GetUnspentByHeight(programHash, assetId, height)
				if err != nil {
					return errors.New(fmt.Sprintf("[programHash:%v, assetId:%v height: %v] has no unspent UTXO.",
						programHash, assetId, height))
				}
			}

			flag := false
			listnum := len(unspents[programHash][assetId][height])
			for i := 0; i < listnum; i++ {
				if unspents[programHash][assetId][height][i].Txid.CompareTo(input.ReferTxID) == 0 &&
					unspents[programHash][assetId][height][i].Index == uint32(input.ReferTxOutputIndex) {
					unspents[programHash][assetId][height][i] = unspents[programHash][assetId][height][listnum-1]
					unspents[programHash][assetId][height] = unspents[programHash][assetId][height][:listnum-1]
					flag = true
					break
				}
			}

			if !flag {
				return errors.New("utxoUnspents NOT find UTXO by txid")
			}

		}

		for i, output := range tx.Outputs {
			programHash := output.ProgramHash
			assetId := output.AssetID
			unspent := &transaction.UTXOUnspent{
				Txid:  tx.Hash(),
				Index: uint32(i),
				Value: output.Value,
			}

			//utxos.addUTXO(programHash, assetId, height, unspent)
			if _, ok := unspents[programHash]; !ok {
				unspents[programHash] = make(map[common.Uint256]map[uint32][]*transaction.UTXOUnspent)
			}

			if _, ok := unspents[programHash][assetId]; !ok {
				unspents[programHash][assetId] = make(map[uint32][]*transaction.UTXOUnspent, 0)
			}

			var err error
			if _, ok := unspents[programHash][assetId][height]; !ok {
				unspents[programHash][assetId][height], err = bs.store.GetUnspentByHeight(programHash, assetId, height)
				if err != nil {
					unspents[programHash][assetId][height] = make([]*transaction.UTXOUnspent, 0)
				}
			}

			unspents[programHash][assetId][height] = append(unspents[programHash][assetId][height], unspent)
		}

	}

	for programHash, programHash_value := range unspents {
		for assetId, unspents := range programHash_value {
			for height, unspent := range unspents {
				listnum := len(unspent)

				if listnum == 0 {
					if err := bs.batch.BatchDelete(unspentUtxoKey(programHash, assetId, height)); err != nil {
						return err
					}
					continue
				}

				w := bytes.NewBuffer(nil)
				serialization.WriteVarUint(w, uint64(listnum))
				for i := 0; i < listnum; i++ {
					unspent[i].Serialize(w)
				}

				if err := bs.batch.BatchPut(unspentUtxoKey(programHash, assetId, height), w.Bytes()); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

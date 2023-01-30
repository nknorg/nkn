package pool

import (
	"fmt"
	"time"

	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/transaction"
)

// sync txn
const MaxSyncTxnInterval = 16000 // in milliseconds

type txnWithTime struct { // abbreviation is twt: txn with time
	arriveTime int64
	txn        *transaction.Transaction
}

// xor of byte slices.
// notice!!!, b1 will be modified, b1 is the result.
func xorBytes(b1 []byte, b2 []byte) ([]byte, error) {
	if len(b1) != len(b2) {
		return nil, fmt.Errorf("Input slices don't have the same length")
	}

	for i := 0; i < len(b1); i++ {
		b1[i] = b1[i] ^ b2[i]
	}

	return b1, nil
}

// update twtFingerprint
func (tp *TxnPool) updateTwtFingerprint(txnHash common.Uint256) []byte {

	if tp.twtFingerprint == nil {
		tp.twtFingerprint = make([]byte, 0, len(txnHash.ToArray()))
		tp.twtFingerprint = append(tp.twtFingerprint, txnHash.ToArray()...)
	} else {
		tp.twtFingerprint, _ = xorBytes(tp.twtFingerprint, txnHash.ToArray())
	}
	return tp.twtFingerprint
}

// when append new txn to pool, we add this txn pointer to txtWithTime map too.
func (tp *TxnPool) appendTwt(txn *transaction.Transaction) *txnWithTime {

	now := time.Now().UnixMilli()

	twt := &txnWithTime{
		arriveTime: now,
		txn:        txn,
	}

	tp.twtMapMu.Lock()
	defer tp.twtMapMu.Unlock()

	txnHash := txn.Hash()
	_, ok := tp.twtMap.Get(txnHash)
	if !ok {
		_, present := tp.twtMap.Set(txnHash, twt)
		if !present {
			tp.updateTwtFingerprint(txnHash)
			tp.twtCount++
		}
	}

	return twt
}

// remove a txn from twtMap
func (tp *TxnPool) removeTwt(txnHash common.Uint256) (twt *txnWithTime) {
	tp.twtMapMu.Lock()
	defer tp.twtMapMu.Unlock()

	v, ok := tp.twtMap.Delete(txnHash)
	if ok {
		tp.twtCount--
		twt = v.(*txnWithTime)
		txnHash := twt.txn.Hash()
		tp.updateTwtFingerprint(txnHash)
	}

	return
}

// replace an oldtxn by a new txn in twtMap
func (tp *TxnPool) replaceTwt(oldTxn, newTxn *transaction.Transaction) (twt *txnWithTime) {
	if oldTxn == nil || newTxn == nil {
		return
	}

	tp.removeTwt(oldTxn.Hash())
	tp.appendTwt(newTxn)

	return
}

// romve expired items from txn with time list
func (tp *TxnPool) RemoveExpiredTwt() (expiredTwt []*txnWithTime) {

	now := time.Now().UnixMilli()
	expiredThreshold := now - MaxSyncTxnInterval

	delKeys := make([]interface{}, 0)

	tp.twtMapMu.Lock()
	defer tp.twtMapMu.Unlock()

	for pair := tp.twtMap.Oldest(); pair != nil; pair = pair.Next() {
		twt := pair.Value.(*txnWithTime)
		if twt.arriveTime < expiredThreshold { // expired
			delKeys = append(delKeys, pair.Key)
			expiredTwt = append(expiredTwt, pair.Value.(*txnWithTime))
		} else {
			break
		}
	}

	for _, key := range delKeys {
		tp.twtMap.Delete(key)
		tp.twtCount--
		tp.updateTwtFingerprint(key.(common.Uint256))
	}

	return
}

// remove submitted txns from txn with time map
func (tp *TxnPool) removeFromTwtMap(txnsRemoved []*transaction.Transaction) {
	var totalRemoved int = 0

	tp.twtMapMu.Lock()
	defer tp.twtMapMu.Unlock()

	for _, txn := range txnsRemoved {
		txnHash := txn.Hash()
		_, ok := tp.twtMap.Load(txnHash)
		if ok {
			tp.twtMap.Delete(txnHash)
			tp.updateTwtFingerprint(txnHash)
			totalRemoved++
		}
	}
	tp.twtCount -= totalRemoved

}

// get address and nonce from txn with time map
func (tp *TxnPool) GetTwtAddrNonce() map[common.Uint160]uint64 {

	mapAddrNonce := make(map[common.Uint160]uint64)

	tp.twtMapMu.Lock()
	defer tp.twtMapMu.Unlock()

	for pair := tp.twtMap.Oldest(); pair != nil; pair = pair.Next() {

		twt := pair.Value.(*txnWithTime)
		sender, err := twt.txn.GetProgramHashes()
		if err != nil {
			return nil
		}
		nonce, _ := mapAddrNonce[sender[0]]
		if twt.txn.UnsignedTx.Nonce+1 > nonce {
			mapAddrNonce[sender[0]] = twt.txn.UnsignedTx.Nonce + 1 // we send expecected(next) nonce to neighbor
		}
	}

	return mapAddrNonce
}

// get transactions later than specific time from txn with time map
// earliest is in milli-second
func (tp *TxnPool) GetTwtTxnAfter(earliest int64, reqAddrNonce map[common.Uint160]uint64) ([]*pb.Transaction, error) {
	respTxns := make([]*pb.Transaction, 0)

	tp.twtMapMu.Lock()
	defer tp.twtMapMu.Unlock()

	for pair := tp.twtMap.Newest(); pair != nil; pair = pair.Prev() {

		twt := pair.Value.(*txnWithTime)
		if twt.arriveTime < earliest { // skip too old txn
			break
		}

		sender, err := twt.txn.GetProgramHashes()
		if err != nil {
			return nil, err
		}

		addr := sender[0]
		nonce, _ := reqAddrNonce[addr]

		if twt.txn.UnsignedTx.Nonce >= nonce {
			respTxns = append(respTxns, twt.txn.Transaction)
		}
	}

	return respTxns, nil
}

// get xorHashOfTxnTime
func (tp *TxnPool) GetTwtFingerprintAndCount() ([]byte, int) {
	tp.twtMapMu.RLock()
	defer tp.twtMapMu.RUnlock()

	fp := make([]byte, 0, len(tp.twtFingerprint))
	fp = append(fp, tp.twtFingerprint...)

	return fp, tp.twtCount
}

// set last sync time
func (tp *TxnPool) SetSyncTime() {
	tp.lastSyncTime = time.Now().UnixMilli()
}

// set last sync time
func (tp *TxnPool) GetSyncTime() int64 {
	return tp.lastSyncTime
}

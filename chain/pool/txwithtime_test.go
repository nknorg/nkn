package pool

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/transaction"
	"github.com/stretchr/testify/require"
)

// create a rand txn for testing
func createRandTxn(sender *common.Uint160, nonce uint64) *transaction.Transaction {

	rand.Seed(time.Now().UnixMilli())

	recipient := new(common.Uint160)
	randBytes := make([]byte, 20)
	rand.Read(randBytes)
	recipient.SetBytes(randBytes)

	val := rand.Intn(10)
	if val <= 0 {
		val = 1
	}
	value, _ := common.StringToFixed64(fmt.Sprintf("%v", val))
	fee, _ := common.StringToFixed64("0.1")

	txn, _ := transaction.NewTransferAssetTransaction(*sender, *recipient, nonce, value, fee)
	return (txn)
}

// go test -v -run=TestXorBytes
func TestXorBytes(t *testing.T) {
	b1 := []byte{0xff}
	b2 := []byte{0x00}

	b, _ := xorBytes(b1, b2)
	require.Equal(t, byte(0xff), b[0])

	b, _ = xorBytes(b1, b1)
	require.Equal(t, byte(0x00), b[0])

	b, _ = xorBytes(b2, b2)
	require.Equal(t, byte(0x00), b[0])

	b1 = []byte{0xff, 0xff, 0x00}
	b2 = []byte{0x00, 0x00, 0xff}
	b, _ = xorBytes(b1, b2)
	res := bytes.Equal([]byte{0xff, 0xff, 0xff}, b)
	require.Equal(t, true, res)

	b, _ = xorBytes(b1, b1)
	res = bytes.Equal([]byte{0x00, 0x00, 0x00}, b)
	require.Equal(t, true, res)

	// different len
	b1 = []byte{0xff, 0xff, 0x00}
	b2 = []byte{0x00, 0x00}
	b, err := xorBytes(b1, b2)
	require.Equal(t, 0, len(b))
	require.NotEqual(t, nil, err)

}

// go test -v -run=TestUpdateTwtFingerprint
func TestUpdateTwtFingerprint(t *testing.T) {
	tp := NewTxPool()

	b := make([]byte, 32)
	b[31] = 0xff
	h1, _ := common.Uint256ParseFromBytes(b)
	fingerprint := tp.updateTwtFingerprint(h1)
	res := bytes.Equal(fingerprint, h1.ToArray())
	require.Equal(t, true, res)

	tp.twtFingerprint = nil
	fingerprint = nil

	rand.Seed(time.Now().UnixMilli())
	const numMsg = 1000
	for i := 0; i < numMsg; i++ {

		rand.Read(b)
		hash, _ := common.Uint256ParseFromBytes(b)
		res = bytes.Equal(b, hash.ToArray())
		require.Equal(t, true, res)

		fp := tp.updateTwtFingerprint(hash)

		if fingerprint == nil {
			fingerprint = make([]byte, 0, len(b))
			fingerprint = append(fingerprint, b...)
		} else {
			fingerprint, _ = xorBytes(fingerprint, b)
		}

		res = bytes.Equal(fp, fingerprint)
		require.Equal(t, true, res)
	}

}

// go test -v -run=TestAppendTwt
func TestAppendTwt(t *testing.T) {
	tp := NewTxPool()

	sender := new(common.Uint160)
	sender.SetBytes([]byte{0x01})

	// test Append one txn
	nonce := uint64(1)
	txn := createRandTxn(sender, nonce)
	txnHash := txn.Hash()

	tp.appendTwt(txn)

	v, ok := tp.twtMap.Get(txnHash)
	require.Equal(t, true, ok)
	twt := v.(*txnWithTime)
	fingerprint := txnHash.ToArray()

	require.Equal(t, txn, twt.txn)
	require.Equal(t, 1, tp.twtCount)
	require.Equal(t, fingerprint, tp.twtFingerprint)

	// remove it, reset
	tp.removeTwt(txnHash)
	fingerprint = nil

	// test append batch txn in ordered.
	const numTxn = 1000
	txnAppended := make([]*transaction.Transaction, 0, numTxn)
	for i := 0; i < numTxn; i++ {
		nonce++
		txn := createRandTxn(sender, nonce)
		txnHash := txn.Hash()
		tp.appendTwt(txn)
		txnAppended = append(txnAppended, txn)
		b := txnHash.ToArray()
		if fingerprint == nil {
			fingerprint = make([]byte, 0, len(b))
			fingerprint = append(fingerprint, b...)
		} else {
			fingerprint, _ = xorBytes(fingerprint, b)
		}
	}
	require.Equal(t, numTxn, tp.twtCount)
	require.Equal(t, fingerprint, tp.twtFingerprint)

	// pop out twt from oldest to newest, they should be same order as we appened them.
	for i := 0; i < numTxn; i++ {
		pair := tp.twtMap.Oldest()
		twt := pair.Value.(*txnWithTime)
		require.Equal(t, txnAppended[i], twt.txn)
		tp.twtMap.Delete(pair.Key)
	}

}

// go test -v -run=TestRemoveTwt
func TestRemoveTwt(t *testing.T) {
	tp := NewTxPool()

	sender := new(common.Uint160)
	sender.SetBytes([]byte{0x01})

	nonce := uint64(1)
	txn := createRandTxn(sender, nonce)
	txnHash := txn.Hash()
	tp.appendTwt(txn)
	require.Equal(t, 1, tp.twtCount)

	twt := tp.removeTwt(txnHash)

	require.Equal(t, txn, twt.txn)
	require.Equal(t, 0, tp.twtCount)

	const numMsg = 1000
	txnMap := make(map[common.Uint256]*transaction.Transaction)
	for i := 0; i < numMsg; i++ {
		nonce++
		txn := createRandTxn(sender, nonce)
		txnHash := txn.Hash()
		tp.appendTwt(txn)
		txnMap[txnHash] = txn
	}
	require.Equal(t, numMsg, tp.twtCount)

	for txnHash, txn := range txnMap {
		twt := tp.removeTwt(txnHash)
		require.Equal(t, txn, twt.txn)
	}
	require.Equal(t, 0, tp.twtCount)

}

// go test -v -run=TestReplaceTwt
func TestReplaceTwt(t *testing.T) {
	tp := NewTxPool()

	sender := new(common.Uint160)
	sender.SetBytes([]byte{0x01})

	nonce := uint64(1)
	txn1 := createRandTxn(sender, nonce)
	tp.appendTwt(txn1)

	txnHash1 := txn1.Hash()
	v, _ := tp.twtMap.Get(txnHash1)
	twt1 := v.(*txnWithTime)

	require.Equal(t, 1, tp.twtCount)
	require.Equal(t, txn1, twt1.txn)
	require.Equal(t, txnHash1.ToArray(), tp.twtFingerprint)

	// create a new txn with the same sender and nonce
	txn2 := createRandTxn(sender, nonce)

	tp.replaceTwt(txn1, txn2)

	txnHash2 := txn2.Hash()
	v, _ = tp.twtMap.Get(txnHash2)
	twt2 := v.(*txnWithTime)

	require.Equal(t, 1, tp.twtCount) // twtCound should not increased.
	require.Equal(t, txn2, twt2.txn)
	require.Equal(t, txnHash2.ToArray(), tp.twtFingerprint)

}

// go test -v -run=TestRemoveExpiredTwt
func TestRemoveExpiredTwt(t *testing.T) {
	tp := NewTxPool()

	sender := new(common.Uint160)
	sender.SetBytes([]byte{0x01})
	nonce := uint64(1)

	var txns []*transaction.Transaction
	for i := 0; i < 10; i++ { // insert 10 txns in each second
		txn := createRandTxn(sender, nonce)
		nonce++
		tp.appendTwt(txn)
		require.Equal(t, i+1, tp.twtCount)
		txns = append(txns, txn)

		time.Sleep(time.Second)
	}

	time.Sleep((MaxSyncTxnInterval - 10) * time.Millisecond)
	totalExpired := 0
	for i := 0; i < 10; i++ {
		expiredTwt := tp.RemoveExpiredTwt()
		for j := 0; j < len(expiredTwt); j++ { // expired as same as we appended them.
			require.Equal(t, txns[totalExpired], expiredTwt[j].txn)
			totalExpired++
		}
		if totalExpired >= 10 {
			break
		}

		time.Sleep(time.Second)
	}

	require.Equal(t, 0, tp.twtCount)

}

// go test -v -run=TestGetTwtAddrNonce
func TestGetTwtAddrNonce(t *testing.T) {
	tp := NewTxPool()

	sender1 := new(common.Uint160)
	sender1.SetBytes([]byte{0x01})
	nonce1 := uint64(1)

	for i := 0; i < 10; i++ {
		txn := createRandTxn(sender1, nonce1)
		nonce1++
		tp.appendTwt(txn)
	}

	sender2 := new(common.Uint160)
	sender2.SetBytes([]byte{0x02})
	nonce2 := uint64(10)

	for i := 0; i < 5; i++ {
		txn := createRandTxn(sender2, nonce2)
		nonce2++
		tp.appendTwt(txn)
	}

	mapAddrNonce := tp.GetTwtAddrNonce()
	require.Equal(t, 2, len(mapAddrNonce))
	require.Equal(t, nonce1, mapAddrNonce[*sender1])
	require.Equal(t, nonce2, mapAddrNonce[*sender2])

}

// go test -v -run=TestGetTwtTxnAfter
func TestGetTwtTxnAfter(t *testing.T) {
	tp := NewTxPool()

	sender1 := new(common.Uint160)
	sender1.SetBytes([]byte{0x01})
	nonce1 := uint64(1)

	sender2 := new(common.Uint160)
	sender2.SetBytes([]byte{0x02})
	nonce2 := uint64(1)

	now1 := time.Now().UnixMilli()
	for i := 0; i < 5; i++ { // insert 5 txns in each second
		txn := createRandTxn(sender1, nonce1)
		nonce1++
		tp.appendTwt(txn)

		txn = createRandTxn(sender2, nonce2)
		nonce2++
		tp.appendTwt(txn)

		time.Sleep(time.Second)
	}

	now2 := time.Now().UnixMilli()
	time.Sleep(time.Second)

	for i := 0; i < 5; i++ { // insert 5 txns
		txn := createRandTxn(sender2, nonce2)
		nonce2++
		tp.appendTwt(txn)
	}

	// set 1 address, and get after now1
	reqAddrNonce := make(map[common.Uint160]uint64)
	reqAddrNonce[*sender1] = 4

	txnList, err := tp.GetTwtTxnAfter(now1, reqAddrNonce)
	require.Equal(t, nil, err)
	require.Equal(t, 12, len(txnList))

	// set 2 addresses, and get after now1
	reqAddrNonce[*sender1] = 4
	reqAddrNonce[*sender2] = 6

	txnList, err = tp.GetTwtTxnAfter(now1, reqAddrNonce)
	require.Equal(t, nil, err)
	require.Equal(t, 7, len(txnList))

	//  set 2 addresses, get txn after now2
	txnList, err = tp.GetTwtTxnAfter(now2, reqAddrNonce)
	require.Equal(t, nil, err)
	require.Equal(t, 5, len(txnList))

}

// go test -v -run=TestRemoveFromTwtMap
func TestRemoveFromTwtMap(t *testing.T) {
	tp := NewTxPool()

	sender := new(common.Uint160)
	sender.SetBytes([]byte{0x01})
	nonce := uint64(1)

	var txns []*transaction.Transaction
	for i := 0; i < 10; i++ { // insert 10 txns in each second
		txn := createRandTxn(sender, nonce)
		nonce++
		tp.appendTwt(txn)
		require.Equal(t, i+1, tp.twtCount)

		if i%2 == 1 {
			txns = append(txns, txn)
		}
	}
	require.Equal(t, 10, tp.twtCount)

	tp.removeFromTwtMap(txns)

	require.Equal(t, 5, tp.twtCount)
}

// go test -v -run=TestGetTwtFingerprintAndCount
func TestGetTwtFingerprintAndCount(t *testing.T) {
	tp := NewTxPool()

	sender := new(common.Uint160)
	sender.SetBytes([]byte{0x01})
	nonce := uint64(1)

	const numTxn = 10
	for i := 0; i < numTxn; i++ { // insert 10 txns in each second
		txn := createRandTxn(sender, nonce)
		nonce++
		tp.appendTwt(txn)
		require.Equal(t, i+1, tp.twtCount)
	}

	fp, count := tp.GetTwtFingerprintAndCount()
	res := bytes.Equal(fp, tp.twtFingerprint)

	require.Equal(t, numTxn, count)
	require.Equal(t, true, res)
}

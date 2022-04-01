package transaction

import (
	"container/heap"
	"sort"

	"github.com/nknorg/nkn/v2/config"
)

var DefaultCompare = CompareTxnsByFeePerSize

func DefaultSort(s []*Transaction) sort.Interface {
	return SortTxnsByFeePerSize(s)
}

func DefaultHeap(s []*Transaction) heap.Interface {
	return (*SortTxnsByFeePerSize)(&s)
}

func DefaultIsLowFeeTxn(txn *Transaction) bool {
	return IsLowFeePerSizeTxn(txn)
}

type SortTxnsByNonce []*Transaction

func (s SortTxnsByNonce) Len() int           { return len(s) }
func (s SortTxnsByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s SortTxnsByNonce) Less(i, j int) bool { return s[i].UnsignedTx.Nonce < s[j].UnsignedTx.Nonce }

func CompareTxnsByFee(txn1, txn2 *Transaction) int {
	if txn1.UnsignedTx.Fee > txn2.UnsignedTx.Fee {
		return 1
	}
	if txn1.UnsignedTx.Fee < txn2.UnsignedTx.Fee {
		return -1
	}
	if txn1.GetSize() > txn2.GetSize() {
		return -1
	}
	if txn1.GetSize() < txn2.GetSize() {
		return 1
	}
	return 0
}

func IsLowFeeTxn(txn *Transaction) bool {
	return txn.UnsignedTx.Fee < config.Parameters.LowTxnFee
}

type SortTxnsByFee []*Transaction

func (s SortTxnsByFee) Len() int            { return len(s) }
func (s SortTxnsByFee) Swap(i, j int)       { s[i], s[j] = s[j], s[i] }
func (s SortTxnsByFee) Less(i, j int) bool  { return CompareTxnsByFee(s[i], s[j]) < 0 }
func (s *SortTxnsByFee) Push(x interface{}) { *s = append(*s, x.(*Transaction)) }
func (s *SortTxnsByFee) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

func CompareTxnsByFeePerSize(txn1, txn2 *Transaction) int {
	var feePerSize1, feePerSize2 float64
	if txn1.GetSize() > 0 {
		feePerSize1 = float64(txn1.UnsignedTx.Fee) / float64(txn1.GetSize())
	}
	if txn2.GetSize() > 0 {
		feePerSize2 = float64(txn2.UnsignedTx.Fee) / float64(txn2.GetSize())
	}
	if feePerSize1 > feePerSize2 {
		return 1
	}
	if feePerSize1 < feePerSize2 {
		return -1
	}
	return 0
}

func IsLowFeePerSizeTxn(txn *Transaction) bool {
	if txn.GetSize() == 0 {
		return true
	}
	return float64(txn.UnsignedTx.Fee)/float64(txn.GetSize()) < config.Parameters.LowTxnFeePerSize
}

type SortTxnsByFeePerSize []*Transaction

func (s SortTxnsByFeePerSize) Len() int            { return len(s) }
func (s SortTxnsByFeePerSize) Swap(i, j int)       { s[i], s[j] = s[j], s[i] }
func (s SortTxnsByFeePerSize) Less(i, j int) bool  { return CompareTxnsByFeePerSize(s[i], s[j]) < 0 }
func (s *SortTxnsByFeePerSize) Push(x interface{}) { *s = append(*s, x.(*Transaction)) }
func (s *SortTxnsByFeePerSize) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

package transaction

import (
	"container/heap"
	"sort"
)

var DefaultCompare = CompareTxnsByFeePerSize

func DefaultSort(s []*Transaction) sort.Interface {
	return SortTxnsByFeePerSize(s)
}

func DefaultHeap(s []*Transaction) heap.Interface {
	return (*SortTxnsByFeePerSize)(&s)
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
	feePerSize1 := float64(txn1.UnsignedTx.Fee) / (float64(txn1.GetSize()) + 0.01)
	feePerSize2 := float64(txn2.UnsignedTx.Fee) / (float64(txn2.GetSize()) + 0.01)
	if feePerSize1 > feePerSize2 {
		return 1
	}
	if feePerSize1 < feePerSize2 {
		return -1
	}
	return 0
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

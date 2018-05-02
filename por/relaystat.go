package por

import (
	"bytes"
	"errors"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
)

type RelayStat struct {
	PrevBlockHash common.Uint256
	TxPool        []common.Uint256
	Relays        map[string]int
	//TxCommitMerkle common.Uint256
}

func NewStat(PrevBlock common.Uint256) RelayStat {
	rs := RelayStat{
		PrevBlockHash: PrevBlock,
	}

	return rs
}

func (r *RelayStat) CalcRelays(txn *transaction.Transaction) error {
	if txn.TxType != transaction.Commit {
		return errors.New("error transaction type")
	}
	rs := txn.Payload.(*payload.Commit)
	buf := bytes.NewBuffer(rs.SigChain)
	var sigChain SigChain
	sigChain.Deserialize(buf)

	r.TxPool = append(r.TxPool, txn.Hash())

	for _, v := range sigChain.elems {
		r.Relays[common.BytesToHexString(v.pubkey)] += 1
	}

	return nil
}

func (r *RelayStat) GetRelays(pk []byte) int {
	return r.Relays[common.BytesToHexString(pk)]
}

func (r *RelayStat) GetMaxRelay() []byte {
	var pk []byte
	i := 0
	for k, v := range r.Relays {
		if v >= i {
			kb, _ := common.HexStringToBytes(k)
			pk = kb
		}
	}

	return pk
}

func (r *RelayStat) MergeRelays(por IPor) error {
	rs, ok := por.(*RelayStat)
	if !ok {
		return errors.New("error parameters")
	}

	if r.PrevBlockHash != rs.PrevBlockHash {
		return errors.New("error PrevBlockHash")
	}

	r.TxPool = append(r.TxPool, rs.TxPool[:]...)
	for k, v := range rs.Relays {
		r.Relays[k] = v
	}

	return nil
}

func (r *RelayStat) IsRelaying(pk []byte) bool {

	if _, ok := r.Relays[common.BytesToHexString(pk)]; ok {
		return true
	}
	return false
}

func (r *RelayStat) TotalRelays() int {
	total := 0
	for _, v := range r.Relays {
		total += v
	}
	return total
}

func (r *RelayStat) IsTxProcessed(txn *transaction.Transaction) bool {
	txHash := txn.Hash()
	for _, v := range r.TxPool {
		if v == txHash {
			return true
		}
	}
	return false
}

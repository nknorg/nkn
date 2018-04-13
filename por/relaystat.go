package por

import (
	"bytes"
	"errors"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
)

type RelayStat struct {
	PrevBlockHash Uint256
	TxPool        []Uint256
	Relays        map[crypto.PubKey]int
	//TxCommitMerkle Uint256
}

func NewStat(PrevBlock Uint256) IPor {
	rs := &RelayStat{
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
	sigChain.Deserialization(buf)

	r.TxPool = append(r.TxPool, txn.Hash())

	for _, v := range sigChain.elems {
		r.Relays[*v.pubkey] += 1
	}

	return nil
}

func (r *RelayStat) GetRelays(pk *crypto.PubKey) int {
	return r.Relays[*pk]
}

func (r *RelayStat) GetMaxRelay() crypto.PubKey {
	var pk crypto.PubKey
	i := 0
	for k, v := range r.Relays {
		if v >= i {
			pk = k
		}
	}

	return pk
}

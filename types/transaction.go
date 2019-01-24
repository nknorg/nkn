package types

import "github.com/nknorg/nkn/common"

func NewMsgTx(payload *Payload, nonce uint64, fee common.Fixed64, attrs []byte) *MsgTx {
	unsigned := &UnsignedTx{
		Payload:    payload,
		Nonce:      nonce,
		Fee:        int64(fee),
		Attributes: attrs,
	}

	tx := &MsgTx{
		UnsignedTx: unsigned,
	}

	return tx
}

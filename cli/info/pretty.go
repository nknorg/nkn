package info

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/nknorg/nkn/pb"
)

type PrettyPrinter []byte

func TxnUnmarshal(m map[string]interface{}) (interface{}, error) {
	typ, ok := m["txType"]
	if !ok {
		return nil, fmt.Errorf("No such key [txType]")
	}

	pbHexStr, ok := m["payloadData"]
	if !ok {
		return m, nil
	}
	buf, err := hex.DecodeString(pbHexStr.(string))

	switch typ {
	case pb.PayloadType_name[int32(pb.SIG_CHAIN_TXN_TYPE)]:
		sigChainTxn := &pb.SigChainTxn{}
		if err = sigChainTxn.Unmarshal(buf); err == nil { // bin to pb struct of SigChainTxnType txn
			m["payloadData"] = sigChainTxn.ToMap()
		}
	case pb.PayloadType_name[int32(pb.COINBASE_TYPE)]:
		coinBaseTxn := &pb.Coinbase{}
		if err = coinBaseTxn.Unmarshal(buf); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = coinBaseTxn.ToMap()
		}
	case pb.PayloadType_name[int32(pb.TRANSFER_ASSET_TYPE)]:
		trans := &pb.TransferAsset{}
		if err = trans.Unmarshal(buf); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = trans.ToMap()
		}
	case pb.PayloadType_name[int32(pb.GENERATE_ID_TYPE)]:
		genID := &pb.GenerateID{}
		if err = genID.Unmarshal(buf); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = genID.ToMap()
		}
	case pb.PayloadType_name[int32(pb.REGISTER_NAME_TYPE)]:
		regName := &pb.RegisterName{}
		if err = regName.Unmarshal(buf); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = regName.ToMap()
		}
	case pb.PayloadType_name[int32(pb.SUBSCRIBE_TYPE)]:
		sub := &pb.Subscribe{}
		if err = sub.Unmarshal(buf); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = sub.ToMap()
		}
	case pb.PayloadType_name[int32(pb.NANO_PAY_TYPE)]:
		pay := &pb.NanoPay{}
		if err = pay.Unmarshal(buf); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = pay.ToMap()
		}
	case pb.PayloadType_name[int32(pb.TRANSFER_NAME_TYPE)]:
		fallthrough //TODO
	case pb.PayloadType_name[int32(pb.DELETE_NAME_TYPE)]:
		fallthrough //TODO
	case pb.PayloadType_name[int32(pb.UNSUBSCRIBE_TYPE)]:
		fallthrough //TODO
	case pb.PayloadType_name[int32(pb.ISSUE_ASSET_TYPE)]:
		fallthrough //TODO
	default:
		return nil, fmt.Errorf("Unknow txType[%s] for pretty print", typ)
	}

	return m, nil
}

func (resp PrettyPrinter) PrettyTxn() ([]byte, error) {
	m := map[string]interface{}{}
	err := json.Unmarshal(resp, &m)
	if err != nil {
		return nil, err
	}

	v, ok := m["result"]
	if !ok {
		return nil, fmt.Errorf("response No such key [result]")
	}

	if m["result"], err = TxnUnmarshal(v.(map[string]interface{})); err != nil {
		return nil, err
	}

	return json.Marshal(m)
}

func (resp PrettyPrinter) PrettyBlock() ([]byte, error) {
	m := map[string]interface{}{}
	err := json.Unmarshal(resp, &m)
	if err != nil {
		return nil, err
	}

	ret, ok := m["result"]
	if !ok {
		return nil, fmt.Errorf("response No such key [result]")
	}

	txns, ok := ret.(map[string]interface{})["transactions"]
	if !ok {
		return nil, fmt.Errorf("result No such key [transactions]")
	}

	lst := make([]interface{}, 0)
	for _, t := range txns.([]interface{}) {
		if m, err := TxnUnmarshal(t.(map[string]interface{})); err == nil {
			lst = append(lst, m)
		} else {
			lst = append(lst, t) // append origin txn if TxnUnmarshal fail
		}
	}
	txns = lst

	return json.Marshal(m)
}

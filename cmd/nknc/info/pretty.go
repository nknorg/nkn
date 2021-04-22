package info

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/pb"
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
	if err != nil {
		return nil, err
	}

	switch typ {
	case pb.PayloadType_name[int32(pb.PayloadType_SIG_CHAIN_TXN_TYPE)]:
		sigChainTxn := &pb.SigChainTxn{}
		if err = proto.Unmarshal(buf, sigChainTxn); err == nil { // bin to pb struct of SigChainTxnType txn
			m["payloadData"] = sigChainTxn.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_COINBASE_TYPE)]:
		coinBaseTxn := &pb.Coinbase{}
		if err = proto.Unmarshal(buf, coinBaseTxn); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = coinBaseTxn.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_TRANSFER_ASSET_TYPE)]:
		trans := &pb.TransferAsset{}
		if err = proto.Unmarshal(buf, trans); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = trans.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_GENERATE_ID_TYPE)]:
		genID := &pb.GenerateID{}
		if err = proto.Unmarshal(buf, genID); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = genID.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_REGISTER_NAME_TYPE)]:
		regName := &pb.RegisterName{}
		if err = proto.Unmarshal(buf, regName); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = regName.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_SUBSCRIBE_TYPE)]:
		sub := &pb.Subscribe{}
		if err = proto.Unmarshal(buf, sub); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = sub.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_UNSUBSCRIBE_TYPE)]:
		sub := &pb.Unsubscribe{}
		if err = proto.Unmarshal(buf, sub); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = sub.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_NANO_PAY_TYPE)]:
		pay := &pb.NanoPay{}
		if err = proto.Unmarshal(buf, pay); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = pay.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_TRANSFER_NAME_TYPE)]:
		fallthrough //TODO
	case pb.PayloadType_name[int32(pb.PayloadType_DELETE_NAME_TYPE)]:
		fallthrough //TODO
	case pb.PayloadType_name[int32(pb.PayloadType_ISSUE_ASSET_TYPE)]:
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
	m["transactions"] = lst

	return json.Marshal(m)
}

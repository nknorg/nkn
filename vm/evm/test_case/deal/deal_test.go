package deal

import (
	"testing"
	"DNA/vm/evm/test_case"
	"DNA/common"
	"fmt"
)

const (
	ABI = `[{"constant":false,"inputs":[],"name":"getSignerBalance","outputs":[{"name":"balance","type":"uint256"}],"payable":false,"type":"function"},{"constant":false,"inputs":[],"name":"getReceiverBalance","outputs":[{"name":"balance","type":"uint256"}],"payable":false,"type":"function"},{"constant":false,"inputs":[],"name":"getSigner","outputs":[{"name":"retVal","type":"address"}],"payable":false,"type":"function"},{"constant":false,"inputs":[],"name":"getReceiver","outputs":[{"name":"retVal","type":"address"}],"payable":false,"type":"function"},{"constant":false,"inputs":[],"name":"releasePayment","outputs":[],"payable":false,"type":"function"},{"constant":true,"inputs":[],"name":"receiver","outputs":[{"name":"","type":"address"}],"payable":false,"type":"function"},{"inputs":[],"payable":false,"type":"constructor"}]`
	BIN = `6060604052341561000c57fe5b5b33600060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550734e2d29c060eefc41a4f5330ecb67da94e0eed266600160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055505b5b6103ca806100b46000396000f30060606040523615610076576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680630d405c581461007857806310fb7bc61461009e5780637ac3c02f146100c457806398aca92214610116578063d116c8c414610168578063f7260d3e1461017a575bfe5b341561008057fe5b6100886101cc565b6040518082815260200191505060405180910390f35b34156100a657fe5b6100ae61020e565b6040518082815260200191505060405180910390f35b34156100cc57fe5b6100d4610250565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b341561011e57fe5b61012661027b565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b341561017057fe5b6101786102a6565b005b341561018257fe5b61018a610378565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b6000600060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163190505b90565b6000600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163190505b90565b6000600060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1690505b90565b6000600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1690505b90565b3373ffffffffffffffffffffffffffffffffffffffff16600060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415156103035760006000fd5b600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166108fc3073ffffffffffffffffffffffffffffffffffffffff16319081150290604051809050600060405180830381858888f19350505050505b565b600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16815600a165627a7a723058206c5dadb232323c6fde39fb4eb69a4df3e260f9a357f5a01434e2c61977b80dcb0029`
)

func TestGetSigner(t *testing.T) {
	t.Log("testing greet start")
	codeHash, account, evm, parsed, err := test_case.NewEngine(ABI, BIN)
	if err != nil { t.Errorf("new engine error:%v", err)}
	input, err := parsed.Pack("getSigner")
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err := evm.Call(*account, *codeHash, input)
	ret0 := new(common.Uint160)
	err = parsed.Unpack(ret0, "getSigner", ret)
	if err != nil { t.Errorf("unpack error:%v", err)}
	t.Log("ret0:", *ret0)
	t.Log("testing greet end")
}

func TestGetReceiver(t *testing.T) {
	t.Log("testing greet start")
	codeHash, account, evm, parsed, err := test_case.NewEngine(ABI, BIN)
	if err != nil { t.Errorf("new engine error:%v", err)}
	input, err := parsed.Pack("getReceiver")
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err := evm.Call(*account, *codeHash, input)
	ret1 := new(common.Uint160)
	err = parsed.Unpack(ret1, "getReceiver", ret)
	if err != nil { t.Errorf("unpack error:%v", err)}
	t.Log(fmt.Sprintf("ret1:%x", *ret1))
	t.Log("testing greet end")
}

func TestReleasePayment(t *testing.T) {
	t.Log("testing greet start")
	codeHash, account, evm, parsed, err := test_case.NewEngine(ABI, BIN)
	if err != nil { t.Errorf("new engine error:%v", err)}
	input, err := parsed.Pack("releasePayment")
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err := evm.Call(*account, *codeHash, input)
	t.Log("ret3:", ret)
	//ret2 := new(common.Uint160)
	//err = parsed.Unpack(ret2, "releasePayment", ret)
	//if err != nil { t.Errorf("unpack error:%v", err)}
	//t.Log(fmt.Sprintf("ret1:%x", *ret2))
	t.Log("testing greet end")
}


func TestReceiverBalance(t *testing.T) {
	t.Log("testing receiver balance")
	codeHash, account, evm, parsed, err := test_case.NewEngine(ABI, BIN)
	if err != nil { t.Errorf("new engine error:%v", err)}
	input, err := parsed.Pack("getReceiverBalance")
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err := evm.Call(*account, *codeHash, input)
	t.Log("ret4:", ret)
	//ret2 := new(common.Uint160)
	//err = parsed.Unpack(ret2, "releasePayment", ret)
	//if err != nil { t.Errorf("unpack error:%v", err)}
	//t.Log(fmt.Sprintf("ret1:%x", *ret2))
	t.Log("testing greet end")
}

func TestSignerBalance(t *testing.T) {
	t.Log("testing receiver balance")
	codeHash, account, evm, parsed, err := test_case.NewEngine(ABI, BIN)
	if err != nil { t.Errorf("new engine error:%v", err)}
	input, err := parsed.Pack("getSignerBalance")
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err := evm.Call(*account, *codeHash, input)
	t.Log("ret5:", ret)
	//ret2 := new(common.Uint160)
	//err = parsed.Unpack(ret2, "releasePayment", ret)
	//if err != nil { t.Errorf("unpack error:%v", err)}
	//t.Log(fmt.Sprintf("ret1:%x", *ret2))
	t.Log("testing greet end")
}

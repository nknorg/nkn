package coin

import (
	"testing"
	"DNA/vm/evm/test_case"
	"DNA/client"
	"fmt"
	"math/big"
)

const (
	ABI = `[{"constant":true,"inputs":[],"name":"minter","outputs":[{"name":"","type":"address"}],"payable":false,"type":"function"},{"constant":true,"inputs":[{"name":"","type":"address"}],"name":"balances","outputs":[{"name":"","type":"uint256"}],"payable":false,"type":"function"},{"constant":false,"inputs":[{"name":"receiver","type":"address"},{"name":"amount","type":"uint256"}],"name":"mint","outputs":[],"payable":false,"type":"function"},{"constant":false,"inputs":[{"name":"receiver","type":"address"},{"name":"amount","type":"uint256"}],"name":"send","outputs":[],"payable":false,"type":"function"},{"constant":false,"inputs":[{"name":"addr","type":"address"}],"name":"getBalance","outputs":[{"name":"amount","type":"uint256"}],"payable":false,"type":"function"},{"inputs":[],"payable":false,"type":"constructor"},{"anonymous":false,"inputs":[{"indexed":false,"name":"from","type":"address"},{"indexed":false,"name":"to","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"name":"Sent","type":"event"}]`
	BIN = `6060604052341561000c57fe5b5b33600060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055505b5b6104b78061005f6000396000f30060606040526000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063075461721461006757806327e235e3146100b957806340c10f1914610103578063d0679d3414610142578063f8b2cb4f14610181575bfe5b341561006f57fe5b6100776101cb565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34156100c157fe5b6100ed600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506101f1565b6040518082815260200191505060405180910390f35b341561010b57fe5b610140600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091908035906020019091905050610209565b005b341561014a57fe5b61017f600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919080359060200190919050506102b7565b005b341561018957fe5b6101b5600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050610441565b6040518082815260200191505060405180910390f35b600060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60016020528060005260406000206000915090505481565b600060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515610265576102b3565b80600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055505b5050565b80600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410156103035761043d565b80600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254039250508190555080600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055507f3990db2d31862302a685e8086b5755072a6e2b5b780af1ee81ece35ee3cd3345338383604051808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001828152602001935050505060405180910390a15b5050565b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490505b9190505600a165627a7a7230582067e94cc6b43734c17b9205e4f12f59a693bc7e3f29269dcab79013f764268df00029`
)

func TestCoin(t *testing.T) {
	codeHash, account, evm, parsed, err := test_case.NewEngine(ABI, BIN)
	if err != nil { t.Errorf("new engine error:%v", err)}

	account1, _ := client.NewAccount()
	account2, _ := client.NewAccount()
	account3, _ := client.NewAccount()
	//account4, _ := client.NewAccount()

	input, err := parsed.Pack("mint", account1.ProgramHash, big.NewInt(100))
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err := evm.Call(*account, *codeHash, input)
	fmt.Println("ret0", ret)

	input, err = parsed.Pack("mint", account2.ProgramHash, big.NewInt(100))
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(*account, *codeHash, input)
	fmt.Println("ret1", ret)

	input, err = parsed.Pack("mint", account, big.NewInt(100))
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(*account, *codeHash, input)
	fmt.Println("ret1", ret)

	input, err = parsed.Pack("getBalance", account1.ProgramHash)
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(*account, *codeHash, input)
	fmt.Println("ret2", ret)
	var i *big.Int
	err = parsed.Unpack(&i, "getBalance", ret)
	if err != nil { t.Errorf("unpack error:%v", err)}
	fmt.Println("ret2", i)

	input, err = parsed.Pack("send", account3.ProgramHash, big.NewInt(10))
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(*account, *codeHash, input)
	fmt.Println("ret3", ret)

	input, err = parsed.Pack("getBalance", account3.ProgramHash)
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(*account, *codeHash, input)
	fmt.Println("ret4", ret)
	err = parsed.Unpack(&i, "getBalance", ret)
	if err != nil { t.Errorf("unpack error:%v", err)}
	fmt.Println("re4", i)

	input, err = parsed.Pack("getBalance", account)
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(*account, *codeHash, input)
	fmt.Println("ret4", ret)
	err = parsed.Unpack(&i, "getBalance", ret)
	if err != nil { t.Errorf("unpack error:%v", err)}
	fmt.Println("re4", i)

}

package ballot

import (
	"testing"
	"DNA/vm/evm/test_case"
	"fmt"
	"math/big"
	"DNA/client"
)

const (
	ABI = `[{"constant":false,"inputs":[{"name":"proposal","type":"uint256"}],"name":"vote","outputs":[],"payable":false,"type":"function"},{"constant":true,"inputs":[{"name":"","type":"uint256"}],"name":"proposals","outputs":[{"name":"name","type":"bytes32"},{"name":"voteCount","type":"uint256"}],"payable":false,"type":"function"},{"constant":true,"inputs":[],"name":"chairperson","outputs":[{"name":"","type":"address"}],"payable":false,"type":"function"},{"constant":false,"inputs":[{"name":"to","type":"address"}],"name":"delegate","outputs":[],"payable":false,"type":"function"},{"constant":true,"inputs":[],"name":"winningProposal","outputs":[{"name":"winningProposal","type":"uint256"}],"payable":false,"type":"function"},{"constant":false,"inputs":[{"name":"voter","type":"address"}],"name":"giveRightToVote","outputs":[],"payable":false,"type":"function"},{"constant":true,"inputs":[{"name":"","type":"address"}],"name":"voters","outputs":[{"name":"weight","type":"uint256"},{"name":"voted","type":"bool"},{"name":"delegate","type":"address"},{"name":"vote","type":"uint256"}],"payable":false,"type":"function"},{"constant":true,"inputs":[],"name":"winnerCount","outputs":[{"name":"count","type":"uint256"}],"payable":false,"type":"function"},{"constant":true,"inputs":[],"name":"winnerName","outputs":[{"name":"winningName","type":"bytes32"}],"payable":false,"type":"function"},{"inputs":[{"name":"proposalNames","type":"bytes32[]"}],"payable":false,"type":"constructor"}]`
	BIN = `6060604052341561000c57fe5b604051610b31380380610b31833981016040528080518201919050505b600033600060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550600160016000600060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000181905550600090505b815181101561016b57600280548060010182816100f89190610173565b916000526020600020906002020160005b604060405190810160405280868681518110151561012357fe5b906020019060200201516000191681526020016000815250909190915060008201518160000190600019169055602082015181600101555050505b80806001019150506100db565b5b50506101d5565b8154818355818115116101a05760020281600202836000526020600020918201910161019f91906101a5565b5b505050565b6101d291905b808211156101ce57600060008201600090556001820160009055506002016101ab565b5090565b90565b61094d806101e46000396000f30060606040523615610097576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680630121b93f14610099578063013cf08b146100b95780632e4176cf146100fc5780635c19a95c1461014e578063609ff1bd146101845780639e7b8d61146101aa578063a3ec138d146101e0578063caa02e081461026f578063e2ba53f014610295575bfe5b34156100a157fe5b6100b760048080359060200190919050506102c3565b005b34156100c157fe5b6100d76004808035906020019091905050610386565b6040518083600019166000191681526020018281526020019250505060405180910390f35b341561010457fe5b61010c6103ba565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b341561015657fe5b610182600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506103e0565b005b341561018c57fe5b6101946106d3565b6040518082815260200191505060405180910390f35b34156101b257fe5b6101de600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190505061075a565b005b34156101e857fe5b610214600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190505061085c565b60405180858152602001841515151581526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200182815260200194505050505060405180910390f35b341561027757fe5b61027f6108b9565b6040518082815260200191505060405180910390f35b341561029d57fe5b6102a56108ed565b60405180826000191660001916815260200191505060405180910390f35b6000600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002090508060010160009054906101000a900460ff161515156103255760006000fd5b60018160010160006101000a81548160ff021916908315150217905550818160020181905550806000015460028381548110151561035f57fe5b906000526020600020906002020160005b50600101600082825401925050819055505b5050565b60028181548110151561039557fe5b906000526020600020906002020160005b915090508060000154908060010154905082565b600060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60006000600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002091508160010160009054906101000a900460ff161515156104445760006000fd5b3373ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff16141515156104805760006000fd5b5b600073ffffffffffffffffffffffffffffffffffffffff16600160008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010160019054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415156105bf57600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010160019054906101000a900473ffffffffffffffffffffffffffffffffffffffff1692503373ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff16141515156105ba5760006000fd5b610481565b60018260010160006101000a81548160ff021916908315150217905550828260010160016101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002090508060010160009054906101000a900460ff16156106b65781600001546002826002015481548110151561068f57fe5b906000526020600020906002020160005b50600101600082825401925050819055506106cd565b816000015481600001600082825401925050819055505b5b505050565b60006000600060009150600090505b60028054905081101561075457816002828154811015156106ff57fe5b906000526020600020906002020160005b506001015411156107465760028181548110151561072a57fe5b906000526020600020906002020160005b506001015491508092505b5b80806001019150506106e2565b5b505090565b600060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161480156108045750600160008273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010160009054906101000a900460ff16155b15156108105760006000fd5b6001600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600001819055505b50565b60016020528060005260406000206000915090508060000154908060010160009054906101000a900460ff16908060010160019054906101000a900473ffffffffffffffffffffffffffffffffffffffff16908060020154905084565b600060026108c56106d3565b8154811015156108d157fe5b906000526020600020906002020160005b506001015490505b90565b600060026108f96106d3565b81548110151561090557fe5b906000526020600020906002020160005b506000015490505b905600a165627a7a723058208879ee0c993f9f05c97a82355a2500a2c5cf885110540078b7ef5aaaa6dc51260029`
)

func TestGetSigner(t *testing.T) {
	t.Log("testing greet start")

	codeHash, account, evm, parsed, err := test_case.NewEngine(ABI, BIN, [][32]byte{[32]byte{1}, [32]byte{2}, [32]byte{3}})
	if err != nil { t.Errorf("new engine error:%v", err)}
	account1, _ := client.NewAccount()
	account2, _ := client.NewAccount()
	account3, _ := client.NewAccount()
	account4, _ := client.NewAccount()

	fmt.Println("deploy finished.")
	input, err := parsed.Pack("giveRightToVote", account1.ProgramHash)
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err := evm.Call(*account, *codeHash, input)

	input, err = parsed.Pack("giveRightToVote", account2.ProgramHash)
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(*account, *codeHash, input)

	input, err = parsed.Pack("giveRightToVote", account3.ProgramHash)
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(*account, *codeHash, input)

	input, err = parsed.Pack("giveRightToVote", account4.ProgramHash)
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(*account, *codeHash, input)

	input, err = parsed.Pack("vote",  big.NewInt(1))
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(*account, *codeHash, input)
	//
	input, err = parsed.Pack("vote",  big.NewInt(2))
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(account1.ProgramHash, *codeHash, input)
	////
	input, err = parsed.Pack("vote",  big.NewInt(2))
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(account2.ProgramHash, *codeHash, input)
	//
	//input, err = parsed.Pack("vote",  big.NewInt(2))
	//if err != nil { t.Errorf("pack error:%v", err)}
	//ret, err = evm.Call(*account, *codeHash, input)
	//
	//input, err = parsed.Pack("vote",  big.NewInt(2))
	//if err != nil { t.Errorf("pack error:%v", err)}
	//ret, err = evm.Call(*account, *codeHash, input)
	//
	//input, err = parsed.Pack("vote",  big.NewInt(1))
	//if err != nil { t.Errorf("pack error:%v", err)}
	//ret, err = evm.Call(*account, *codeHash, input)
	//
	//input, err = parsed.Pack("vote",  big.NewInt(1))
	//if err != nil { t.Errorf("pack error:%v", err)}
	//ret, err = evm.Call(*account, *codeHash, input)

	input, err = parsed.Pack("winningProposal")
	if err != nil { t.Errorf("pack error:%v", err)}
	ret, err = evm.Call(*account, *codeHash, input)
	fmt.Println("winningProposal ret:", ret)

	input, err = parsed.Pack("winnerName")
	ret, err = evm.Call(*account, *codeHash, input)
	fmt.Println("winnerName ret:", ret)
	//var ret0 [32]byte
	//err = parsed.Unpack(&ret0, "winnerName", ret)
	//if err != nil { t.Errorf("unpack error:%v", err)}
	//fmt.Println("ret0:", ret0)


	input, err = parsed.Pack("winnerCount")
	ret, err = evm.Call(*account, *codeHash, input)
	fmt.Println("winnerCount ret:", ret)
	//err = parsed.Unpack(&1, "winnerName", ret)
	//if err != nil { t.Errorf("unpack error:%v", err)}
	//t.Log("ret0:", ret0)
	t.Log("testing greet end")
}


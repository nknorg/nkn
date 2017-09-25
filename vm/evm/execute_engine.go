package evm

import (
	"DNA/common"
	"sync/atomic"
	"fmt"
	"math/big"
	"DNA/smartcontract/storage"
)

type ExecutionEngine struct {
	contract *Contract
	opCode OpCode
	stack *Stack
	pc uint64
	memory *Memory
	DBCache storage.DBCache
	abort int32
	JumpTable [256]OpExec
	time *big.Int
	blockNumber *big.Int
}

func NewExecutionEngine(dbCache storage.DBCache, time *big.Int, blockNumber *big.Int, gas common.Fixed64) *ExecutionEngine {
	return &ExecutionEngine {
		DBCache: dbCache,
		time: time,
		blockNumber: blockNumber,
		JumpTable: NewOpExecList(),
	}
}

func (e *ExecutionEngine) Create(caller common.Uint160, code []byte) (ret []byte, err error) {
	e.contract = NewContract(caller)
	codeHash, err := common.ToCodeHash(code)
	if err != nil { return nil, err }
	e.contract.SetCode(code, codeHash)
	ret, err = e.run()
	if err != nil { return nil, err }
	e.DBCache.SetCode(codeHash, ret)
	return ret, nil
}

func (e *ExecutionEngine) Call(caller common.Uint160, codeHash common.Uint160, input []byte) (ret []byte, err error) {
	e.contract = NewContract(caller)
	code, err := e.DBCache.GetCode(codeHash)
	if err != nil {
		return nil, err
	}
	e.contract.SetCallCode(code, input, codeHash)
	ret, err = e.run()
	if err != nil { return nil, err }
	return ret, nil
}

func (e *ExecutionEngine) CallCode(codeHash common.Uint160, input []byte) (ret []byte, err error) {
	return ret, nil
}

func (e *ExecutionEngine) DelegateCall(codeHash common.Uint160, toAddr common.Uint160, input []byte) (ret []byte, err error) {
	return ret, nil
}

func(e *ExecutionEngine) run() ([]byte, error) {
	if v, ok := PrecompiledContracts[e.contract.CodeHash]; ok {
		return v.Run(e.contract.Input)
	}

	e.stack = newstack()
	e.memory = NewMemory()
	e.pc = uint64(0)

	for atomic.LoadInt32(&e.abort) == 0 {
		op := e.contract.GetOp(e.pc)
		operation := e.JumpTable[op]
		fmt.Println("op:", operation.Name)

		if operation.Exec == nil {
			return nil, fmt.Errorf("invalid opcode 0x%x", op)
		}

		if err := operation.validateStack(e.stack); err != nil {
			return nil, err
		}

		res, err := operation.Exec(e)

		switch {
		case err != nil:
			return nil, err
		case operation.halts:
			return res, nil
		case !operation.jumps:
			e.pc++
		}
		s := e.stack.len()
		for i:=0; i<s;i++ {
			fmt.Print(" ", e.stack.Back(i))
		}
		fmt.Println()
		fmt.Println("pc:", e.pc)

	}
	return nil, nil
}

package evm

import (
	"math/big"
	"fmt"
	"DNA/vm/evm/common"
	"DNA/vm/evm/crypto"
	. "DNA/common"
)

var (
	bigZero = new(big.Int)
)

func opStop(e *ExecutionEngine) ([]byte, error) {
	return nil, nil
}

func opAdd(e *ExecutionEngine) ([]byte, error) {
	x, y := pop(e), pop(e)
	push(e, common.U256(x.Add(x, y)))
	return nil, nil
}

func opMul(e *ExecutionEngine) ([]byte, error) {
	x, y := pop(e), pop(e)
	push(e, common.U256(x.Mul(x, y)))
	return nil, nil
}

func opSub(e *ExecutionEngine) ([]byte, error) {
	x, y := pop(e), pop(e)
	push(e, common.U256(x.Sub(x, y)))
	return nil, nil
}

func opDiv(e *ExecutionEngine) ([]byte, error) {
	x, y := pop(e), pop(e)
	if y.Sign() != 0 {
		push(e, common.U256(x.Div(x, y)))
	} else {
		push(e, new(big.Int))
	}
	return nil, nil
}

func opSdiv(e *ExecutionEngine) ([]byte, error) {
	x, y := common.S256(pop(e)), common.S256(pop(e))
	if y.Sign() == 0 {
		push(e, new(big.Int))
	} else {
		n := new(big.Int)
		if n.Mul(x, y).Sign() < 0 {
			n.SetInt64(-1)
		} else {
			n.SetInt64(1)
		}
		res := x.Div(x.Abs(x), y.Abs(y))
		res.Mul(res, n)
		push(e, common.U256(res))
	}
	return nil, nil
}

func opMod(e *ExecutionEngine) ([]byte, error) {
	x, y := pop(e), pop(e)
	if y.Sign() == 0 {
		push(e, new(big.Int))
	} else {
		push(e, common.U256(x.Mod(x, y)))
	}
	return nil, nil
}

func opSmod(e *ExecutionEngine) ([]byte, error) {
	x, y := common.S256(pop(e)), common.S256(pop(e))
	if y.Sign() == 0 {
		push(e, new(big.Int))
	} else {
		n := new(big.Int)
		if x.Sign() < 0 {
			n.SetInt64(-1)
		} else {
			n.SetInt64(1)
		}
		res := x.Mod(x.Abs(x), y.Abs(y))
		res.Mul(res, n)
		push(e, common.U256(res))
	}
	return nil, nil
}

func opAddMod(e *ExecutionEngine) ([]byte, error) {
	x, y, z := pop(e), pop(e), pop(e)
	if z.Cmp(bigZero) > 0 {
		add := x.Add(x, y)
		add.Mod(add, z)
		push(e, common.U256(add))
	} else {
		push(e, new(big.Int))
	}
	return nil, nil
}

func opMulMod(e *ExecutionEngine) ([]byte, error) {
	x, y, z := pop(e), pop(e), pop(e)
	if z.Cmp(bigZero) > 0 {
		mul := x.Mul(x, y)
		mul.Mod(mul, z)
		push(e, common.U256(mul))
	} else {
		push(e, new(big.Int))
	}
	return nil, nil
}

func opExp(e *ExecutionEngine) ([]byte, error) {
	base, exponent := pop(e), pop(e)
	push(e, common.Exp(base, exponent))
	return nil, nil
}

func opSignExtend(e *ExecutionEngine) ([]byte, error) {
	back := pop(e)
	if back.Cmp(big.NewInt(31)) < 0 {
		bit := uint(back.Uint64() * 8 + 7)
		num := pop(e)
		mask := back.Lsh(common.Big1, bit)
		mask.Sub(mask, common.Big1)
		if num.Bit(int(bit)) > 0 {
			num.Or(num, mask.Not(mask))
		} else {
			num.And(num, mask)
		}
		push(e, common.U256(num))
	}
	return nil, nil
}

func opLt(e *ExecutionEngine) ([]byte, error) {
	x, y := pop(e), pop(e)
	if x.Cmp(y) < 0 {
		push(e, big.NewInt(1))
	} else {
		push(e, big.NewInt(0))
	}
	return nil, nil
}

func opGt(e *ExecutionEngine) ([]byte, error) {
	x, y := pop(e), pop(e)
	if x.Cmp(y) > 0 {
		push(e, big.NewInt(1))
	} else {
		push(e, big.NewInt(0))
	}
	return nil, nil
}

func opSlt(e *ExecutionEngine) ([]byte, error) {
	x, y := common.S256(pop(e)), common.S256(pop(e))
	if x.Cmp(common.S256(y)) < 0 {
		push(e, big.NewInt(1))
	} else {
		push(e, big.NewInt(0))
	}
	return nil, nil
}

func opSgt(e *ExecutionEngine) ([]byte, error) {
	x, y := common.S256(pop(e)), common.S256(pop(e))
	if x.Cmp(y) > 0 {
		push(e, big.NewInt(1))
	} else {
		push(e, big.NewInt(0))
	}
	return nil, nil
}

func opEq(e *ExecutionEngine) ([]byte, error) {
	x, y := pop(e), pop(e)
	if x.Cmp(y) == 0 {
		push(e, big.NewInt(1))
	} else {
		push(e, big.NewInt(0))
	}
	return nil, nil
}

func opIsZero(e *ExecutionEngine) ([]byte, error) {
	x := pop(e)
	if x.Sign() > 0 {
		push(e, big.NewInt(0))
	} else {
		push(e, big.NewInt(1))
	}
	return nil, nil
}

func opAnd(e *ExecutionEngine) ([]byte, error) {
	x, y := pop(e), pop(e)
	push(e, x.And(x, y))
	return nil, nil
}

func opXor(e *ExecutionEngine) ([]byte, error) {
	x, y := pop(e), pop(e)
	push(e, x.Xor(x, y))
	return nil, nil
}

func opOr(e *ExecutionEngine) ([]byte, error) {
	x, y := pop(e), pop(e)
	push(e, x.Or(x, y))
	return nil, nil
}

func opNot(e *ExecutionEngine) ([]byte, error) {
	x := pop(e)
	push(e, common.U256(x.Not(x)))
	return nil, nil
}

func opByte(e *ExecutionEngine) ([]byte, error) {
	th, val := pop(e), pop(e)
	if th.Cmp(big.NewInt(32)) < 0 {
		b := big.NewInt(int64(common.PaddedBigBytes(val, 32)[th.Int64()]))
		push(e, b)
	} else {
		push(e, big.NewInt(0))
	}
	return nil, nil
}

func opSha3(e *ExecutionEngine) ([]byte, error) {
	offset, size := pop(e), pop(e)
	data := e.memory.Get(offset.Int64(), size.Int64())
	hash := crypto.Keccak256(data)
	push(e, new(big.Int).SetBytes(hash))
	return nil, nil
}

func opAddress(e *ExecutionEngine) ([]byte, error) {
	push(e, e.contract.CodeHash.Big())
	return nil, nil
}

func opBalance(e *ExecutionEngine) ([]byte, error) {
	programHash := BigToUint160(pop(e))
	balance := e.DBCache.GetBalance(programHash)
	push(e, new(big.Int).Set(balance))
	return nil, nil
}

func opOrigin(e *ExecutionEngine) ([]byte, error) {
	return nil, nil
}

func opCaller(e *ExecutionEngine) ([]byte, error) {
	push(e, e.contract.Caller.Big())
	return nil, nil
}

func opCallValue(e *ExecutionEngine) ([]byte, error) {
	push(e, e.contract.value)
	return nil, nil
}

func opCallDataLoad(e *ExecutionEngine) ([]byte, error) {
	push(e, new(big.Int).SetBytes(common.GetData(e.contract.Input, pop(e), common.Big32)))
	return nil, nil
}

func opCallDataSize(e *ExecutionEngine) ([]byte, error) {
	push(e, big.NewInt(int64(len(e.contract.Input))))
	return nil, nil
}

func opCallDataCopy(e *ExecutionEngine) ([]byte, error) {
	var (
		mOff = pop(e)
		cOff = pop(e)
		l = pop(e)
	)
	e.memory.Set(mOff.Uint64(), l.Uint64(), common.GetData(e.contract.Input, cOff, l))
	return nil, nil
}

func opCodeSize(e *ExecutionEngine) ([]byte, error) {
	push(e, big.NewInt(int64(len(e.contract.Code))))
	return nil, nil
}

func opCodeCopy(e *ExecutionEngine) ([]byte, error) {
	var (
		mOff = pop(e)
		cOff = pop(e)
		l = pop(e)
	)
	codeCopy := common.GetData(e.contract.Code, cOff, l)
	e.memory.Set(mOff.Uint64(), l.Uint64(), codeCopy)
	return nil, nil
}

func opGasPrice(e *ExecutionEngine) ([]byte, error) {
	return nil, nil
}

func opExtCodeSize(e *ExecutionEngine) ([]byte, error) {
	a := pop(e)
	codeHash := BigToUint160(a)
	push(e, a.SetInt64(int64(e.DBCache.GetCodeSize(codeHash))))
	return nil, nil
}

func opExtCodeCopy(e *ExecutionEngine) ([]byte, error) {
	var (
		codeHash = BigToUint160(pop(e))
		mOff = pop(e)
		cOff = pop(e)
		l = pop(e)
	)
	state, err := e.DBCache.GetCode(codeHash)
	if err != nil {
		return nil, err
	}
	codeCopy := common.GetData(state, cOff, l)
	e.memory.Set(mOff.Uint64(), l.Uint64(), codeCopy)
	return nil, nil
}

func opBlockHash(e *ExecutionEngine) ([]byte, error) {
	return nil, nil
}

func opCoinBase(e *ExecutionEngine) ([]byte, error) {
	return nil, nil
}

func opTimeStamp(e *ExecutionEngine) ([]byte, error) {
	return nil, nil
}

func opNumber(e *ExecutionEngine) ([]byte, error) {
	return nil, nil
}

func opDifficulty(e *ExecutionEngine) ([]byte, error) {
	return nil, nil
}

func opGasLimit(e *ExecutionEngine) ([]byte, error) {
	return nil, nil
}

func opPop(e *ExecutionEngine) ([]byte, error) {
	pop(e)
	return nil, nil
}

func opMload(e *ExecutionEngine) ([]byte, error) {
	offset := pop(e)
	val := new(big.Int).SetBytes(e.memory.Get(offset.Int64(), 32))
	push(e, val)
	return nil, nil
}

func opMstore(e *ExecutionEngine) ([]byte, error) {
	mStart, val := pop(e), pop(e)
	e.memory.Set(mStart.Uint64(), 32, common.PaddedBigBytes(val, 32))
	return nil, nil
}

func opMstore8(e *ExecutionEngine) ([]byte, error) {
	off, val := pop(e).Int64(), pop(e).Int64()
	e.memory.store[off] = byte(val & 0xff)
	return nil, nil
}

func opSload(e *ExecutionEngine) ([]byte, error) {
	loc := BigToHash(pop(e))
	state, err := e.DBCache.GetState(e.contract.CodeHash, loc)
	if err != nil {
		return nil, err
	}
	push(e, state.Big())
	return nil, nil
}

func opSstore(e *ExecutionEngine) ([]byte, error) {
	loc := BigToHash(pop(e))
	val := pop(e)
	e.DBCache.SetState(e.contract.CodeHash, loc, BigToHash(val))
	return nil, nil
}

func opJump(e *ExecutionEngine) ([]byte, error) {
	pos := pop(e)
	if !e.contract.jumpdest.has(e.contract.CodeHash, e.contract.Code, pos) {
		nop := e.contract.GetOp(pos.Uint64())
		return nil, fmt.Errorf("invalid jump destination (%v) %v", nop, pos)
	}
	e.pc = pos.Uint64()
	return nil, nil
}

func opJumpPi(e *ExecutionEngine) ([]byte, error) {
	pos, cond := pop(e), pop(e)
	if cond.Sign() == 0 {
		e.pc++
	} else {
		if !e.contract.jumpdest.has(e.contract.CodeHash, e.contract.Code, pos) {
			nop := e.contract.GetOp(pos.Uint64())
			return nil, fmt.Errorf("invalid jump destination (%v) %v", nop, pos)
		}
		e.pc = pos.Uint64()
	}
	return nil, nil
}

func opPc(e *ExecutionEngine) ([]byte, error) {
	push(e, new(big.Int).SetUint64(e.pc))
	return nil, nil
}

func opMsize(e *ExecutionEngine) ([]byte, error) {
	push(e, big.NewInt(int64(e.memory.Len())))
	return nil, nil
}

func opGas(e *ExecutionEngine) ([]byte, error) {
	push(e, new(big.Int))
	return nil, nil
}

func opJumpDest(e *ExecutionEngine) ([]byte, error) {
	return nil, nil
}

func opCreate(e *ExecutionEngine) ([]byte, error) {
	pop(e)
	var (
		offset, size = pop(e), pop(e)
		input = e.memory.Get(offset.Int64(), size.Int64())
	)
	engine := NewExecutionEngine(e.DBCache, e.time, e.blockNumber, Fixed64(0))
	_, err := engine.Create(e.contract.Caller, input)
	codeHash, _ := ToCodeHash(input)
	if err != nil {
		push(e, new(big.Int))
	} else {
		push(e, codeHash.Big())
	}
	return nil, nil
}

func opCall(e *ExecutionEngine) ([]byte, error) {
	pop(e)
	hash, value := pop(e), pop(e)
	value = common.U256(value)
	inOffset, inSize := pop(e), pop(e)
	retOffset, retSize := pop(e), pop(e)
	pragramHash := BytesToUint160(hash.Bytes())
	args := e.memory.Get(inOffset.Int64(), inSize.Int64())

	engine := NewExecutionEngine(e.DBCache, e.time, e.blockNumber, Fixed64(0))
	ret, err := engine.Call(pragramHash, e.contract.CodeHash, args)
	if err != nil {
		push(e, new(big.Int))
	} else {
		push(e, big.NewInt(1))
		e.memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	return nil, nil
}

func opCallCode(e *ExecutionEngine) ([]byte, error) {
	opCall(e)
	return nil, nil
}

func opDelegateCall(e *ExecutionEngine) ([]byte, error) {
	opCall(e)
	return nil, nil
}

func opReturn(e *ExecutionEngine) ([]byte, error) {
	offset, size := pop(e), pop(e)
	ret := e.memory.GetPtr(offset.Int64(), size.Int64())
	return ret, nil
}

func opSuicide(e *ExecutionEngine) ([]byte, error) {
	programHash := BigToUint160(pop(e))
	balance := e.DBCache.GetBalance(programHash)
	hash := BigToUint160(pop(e))
	e.DBCache.AddBalance(hash, balance)
	e.DBCache.Suicide(e.contract.CodeHash)
	return nil, nil
}

func makePush(size uint64, pushByteSize int) executionFunc {
	return func(e *ExecutionEngine) ([]byte, error) {
		codeLen := len(e.contract.Code)
		startMin := codeLen
		if int(e.pc + 1) < startMin {
			startMin = int(e.pc + 1)
		}

		endMin := codeLen
		if startMin + pushByteSize < endMin {
			endMin = startMin + pushByteSize
		}

		push(e, new(big.Int).SetBytes(common.RightPadBytes(e.contract.Code[startMin:endMin], pushByteSize)))

		e.pc += size
		return nil, nil
	}
}

func makeDup(size int64) executionFunc {
	return func(e *ExecutionEngine) ([]byte, error) {
		e.stack.dup(int(size))
		return nil, nil
	}
}

func makeSwap(size int64) executionFunc {
	return func(e *ExecutionEngine) ([]byte, error) {
		e.stack.swap(int(size))
		return nil, nil
	}
}

func makeLog(size int) executionFunc {
	return func(e *ExecutionEngine) ([]byte, error) {
		return nil, nil
	}
}

func pop(e *ExecutionEngine) *big.Int {
	return e.stack.pop()
}

func push(e *ExecutionEngine, item *big.Int) {
	e.stack.push(item)
}
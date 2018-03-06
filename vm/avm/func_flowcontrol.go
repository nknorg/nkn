package avm

import (
	. "nkn-core/vm/avm/errors"
	"nkn-core/common/log"
)

func opNop(e *ExecutionEngine) (VMState, error) {
	return NONE, nil
}

func opJmp(e *ExecutionEngine) (VMState, error) {
	offset := int(e.context.OpReader.ReadInt16())

	offset = e.context.GetInstructionPointer() + offset - 3

	if offset > len(e.context.Code) {
		return FAULT, ErrFault
	}
	var (
		fValue = true
	)
	if e.opCode > JMP {
		fValue = PopBoolean(e)

		if e.opCode == JMPIFNOT {
			fValue = !fValue
		}
	}
	if fValue {
		e.context.SetInstructionPointer(int64(offset))
	}
	return NONE, nil
}

func opCall(e *ExecutionEngine) (VMState, error) {
	e.invocationStack.Push(e.context.Clone())
	e.context.SetInstructionPointer(int64(e.context.GetInstructionPointer() + 2))
	e.opCode = JMP
	e.context = e.CurrentContext()
	opJmp(e)
	return NONE, nil
}

func opRet(e *ExecutionEngine) (VMState, error) {
	e.invocationStack.Pop()
	return NONE, nil
}

func opAppCall(e *ExecutionEngine) (VMState, error) {
	codeHash := e.context.OpReader.ReadBytes(20)
	code, err := e.table.GetCode(codeHash)
	if code == nil {
		return FAULT, err
	}
	if e.opCode == TAILCALL {
		e.invocationStack.Pop()
	}
	e.LoadCode(code, false)
	return NONE, nil
}

func opSysCall(e *ExecutionEngine) (VMState, error) {
	s := e.context.OpReader.ReadVarString()

	log.Error("[opSysCall] service name:", s)

	success, err := e.service.Invoke(s, e)
	if success {
		return NONE, nil
	} else {
		return FAULT, err
	}
}


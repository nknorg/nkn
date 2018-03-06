package avm

import (
	. "nkn-core/vm/avm/errors"
	"bytes"
	"encoding/binary"
	"nkn-core/vm/avm/types"
)

func validatorPushData4(e *ExecutionEngine) error {
	index := e.context.GetInstructionPointer()
	if index + 4 >= len(e.context.Code) {
		return ErrOverCodeLen
	}
	bytesBuffer := bytes.NewBuffer(e.context.Code[index: index + 4])
	var l uint32
	binary.Read(bytesBuffer, binary.LittleEndian, &l)
	if l > MaxItemSize {
		return ErrOverMaxItemSize
	}
	return nil
}

func validateCall(e *ExecutionEngine) error {
	if err := validateInvocationStack(e); err != nil {
		return err
	}
	return nil
}

func validateInvocationStack(e *ExecutionEngine) error {
	if uint32(e.invocationStack.Count()) > MaxStackSize {
		return ErrOverStackLen
	}
	return nil
}

func validateAppCall(e *ExecutionEngine) error {
	if err := validateInvocationStack(e); err != nil {
		return err
	}
	if e.table == nil {
		return ErrTableIsNil
	}
	return nil
}

func validateSysCall(e *ExecutionEngine) error {
	if e.service == nil {
		return ErrServiceIsNil
	}
	return nil
}

func validateOpStack(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 1 {
		return ErrUnderStackLen
	}
	index := PeekNInt(0, e)
	if index < 0 {
		return ErrBadValue
	}

	return nil
}

func validateXDrop(e *ExecutionEngine) error {
	if err := validateOpStack(e); err != nil {
		return err
	}
	return nil
}

func validateXSwap(e *ExecutionEngine) error {
	if err := validateOpStack(e); err != nil {
		return err
	}
	return nil
}

func validateXTuck(e *ExecutionEngine) error {
	if err := validateOpStack(e); err != nil {
		return err
	}
	return nil
}

func validatePick(e *ExecutionEngine) error {
	if err := validateOpStack(e); err != nil {
		return err
	}
	return nil
}

func validateRoll(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 1 {
		return ErrUnderStackLen
	}
	index := PeekNInt(0, e)
	if index < 0 {
		return ErrBadValue
	}
	return nil
}

func validateCat(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 2 {
		return ErrUnderStackLen
	}
	l := len(PeekNByteArray(0, e)) + len(PeekNByteArray(1, e))
	if uint32(l) > MaxItemSize {
		return ErrOverMaxItemSize
	}
	return nil
}

func validateSubStr(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 3 {
		return ErrUnderStackLen
	}
	count := PeekNInt(0, e)
	if count < 0 {
		return ErrBadValue
	}
	index := PeekNInt(1, e)
	if index < 0 {
		return ErrBadValue
	}
	arr := PeekNByteArray(2, e)
	if len(arr) < index + count {
		return ErrOverMaxArraySize
	}
	return nil
}

func validateLeft(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 2 {
		return ErrUnderStackLen
	}
	count := PeekNInt(0, e)
	if count < 0 {
		return ErrBadValue
	}
	arr := PeekNByteArray(1, e)
	if len(arr) < count {
		return ErrOverMaxArraySize
	}
	return nil
}

func validateRight(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 2 {
		return ErrUnderStackLen
	}
	count := PeekNInt(0, e)
	if count < 0 {
		return ErrBadValue
	}
	arr := PeekNByteArray(1, e)
	if len(arr) < count {
		return ErrOverMaxArraySize
	}
	return nil
}

func validatorBigIntComp(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 2 {
		return ErrUnderStackLen
	}
	return nil
}

func validatePack(e *ExecutionEngine) error {
	count := PeekInt(e)
	if uint32(count) > MaxArraySize {
		return ErrOverMaxArraySize
	}
	if count > EvaluationStackCount(e) {
		return ErrOverStackLen
	}
	return nil
}

func validatePickItem(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 2 {
		return ErrUnderStackLen
	}
	index := PeekNInt(0, e)
	if index < 0 {
		return ErrBadValue
	}
	item := PeekN(1, e)
	if item == nil {
		return ErrBadValue
	}
	stackItem := item.GetStackItem()
	if _, ok := stackItem.(*types.Array); !ok {
		if _, ok := stackItem.(*types.ByteArray); !ok {
			return ErrNotArray
		} else {
			if index >= len(stackItem.GetByteArray()) {
				return ErrOverMaxArraySize
			}
		}
	} else {
		if index >= len(stackItem.GetArray()) {
			return ErrOverMaxArraySize
		}
	}

	return nil
}

func validatorSetItem(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 3 {
		return ErrUnderStackLen
	}
	newItem := PeekN(0, e)
	if newItem == nil {
		return ErrBadValue
	}
	index := PeekNInt(1, e)
	if index < 0 {
		return ErrBadValue
	}
	arrItem := PeekN(2, e)
	if arrItem == nil {
		return ErrBadValue
	}
	item := arrItem.GetStackItem()
	if _, ok := item.(*types.Array); !ok {
		if _, ok := item.(*types.ByteArray); ok {
			l := len(item.GetByteArray())
			if index >= l {
				return ErrOverMaxArraySize
			}
			if len(newItem.GetStackItem().GetByteArray()) == 0 {
				return ErrBadValue
			}
		} else {
			return ErrNotArray
		}
	}else {
		if index >= len(item.GetArray()) {
			return ErrOverMaxArraySize
		}
	}
	return nil
}

func validateNewArray(e *ExecutionEngine) error {
	count := PeekInt(e)
	if uint32(count) > MaxArraySize {
		return ErrOverMaxArraySize
	}
	return nil
}

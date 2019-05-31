package vm

import (
	"bytes"
	"encoding/binary"

	"github.com/nknorg/nkn/vm/errors"
	"github.com/nknorg/nkn/vm/types"
)

func validatorPushData4(e *ExecutionEngine) error {
	index := e.context.GetInstructionPointer()
	if index+4 >= len(e.context.Code) {
		return errors.ErrOverCodeLen
	}
	bytesBuffer := bytes.NewBuffer(e.context.Code[index : index+4])
	var l uint32
	binary.Read(bytesBuffer, binary.LittleEndian, &l)
	if l > MaxItemSize {
		return errors.ErrOverMaxItemSize
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
		return errors.ErrOverStackLen
	}
	return nil
}

func validateAppCall(e *ExecutionEngine) error {
	if err := validateInvocationStack(e); err != nil {
		return err
	}
	if e.table == nil {
		return errors.ErrTableIsNil
	}
	return nil
}

func validateSysCall(e *ExecutionEngine) error {
	if e.service == nil {
		return errors.ErrServiceIsNil
	}
	return nil
}

func validateOpStack(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 1 {
		return errors.ErrUnderStackLen
	}
	index := PeekNInt(0, e)
	if index < 0 {
		return errors.ErrBadValue
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
		return errors.ErrUnderStackLen
	}
	index := PeekNInt(0, e)
	if index < 0 {
		return errors.ErrBadValue
	}
	return nil
}

func validateCat(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 2 {
		return errors.ErrUnderStackLen
	}
	l := len(PeekNByteArray(0, e)) + len(PeekNByteArray(1, e))
	if uint32(l) > MaxItemSize {
		return errors.ErrOverMaxItemSize
	}
	return nil
}

func validateSubStr(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 3 {
		return errors.ErrUnderStackLen
	}
	count := PeekNInt(0, e)
	if count < 0 {
		return errors.ErrBadValue
	}
	index := PeekNInt(1, e)
	if index < 0 {
		return errors.ErrBadValue
	}
	arr := PeekNByteArray(2, e)
	if len(arr) < index+count {
		return errors.ErrOverMaxArraySize
	}
	return nil
}

func validateLeft(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 2 {
		return errors.ErrUnderStackLen
	}
	count := PeekNInt(0, e)
	if count < 0 {
		return errors.ErrBadValue
	}
	arr := PeekNByteArray(1, e)
	if len(arr) < count {
		return errors.ErrOverMaxArraySize
	}
	return nil
}

func validateRight(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 2 {
		return errors.ErrUnderStackLen
	}
	count := PeekNInt(0, e)
	if count < 0 {
		return errors.ErrBadValue
	}
	arr := PeekNByteArray(1, e)
	if len(arr) < count {
		return errors.ErrOverMaxArraySize
	}
	return nil
}

func validatorBigIntComp(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 2 {
		return errors.ErrUnderStackLen
	}
	return nil
}

func validatePack(e *ExecutionEngine) error {
	count := PeekInt(e)
	if uint32(count) > MaxArraySize {
		return errors.ErrOverMaxArraySize
	}
	if count > EvaluationStackCount(e) {
		return errors.ErrOverStackLen
	}
	return nil
}

func validatePickItem(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 2 {
		return errors.ErrUnderStackLen
	}
	index := PeekNInt(0, e)
	if index < 0 {
		return errors.ErrBadValue
	}
	item := PeekN(1, e)
	if item == nil {
		return errors.ErrBadValue
	}
	stackItem := item.GetStackItem()
	if _, ok := stackItem.(*types.Array); !ok {
		if _, ok := stackItem.(*types.ByteArray); !ok {
			return errors.ErrNotArray
		} else {
			if index >= len(stackItem.GetByteArray()) {
				return errors.ErrOverMaxArraySize
			}
		}
	} else {
		if index >= len(stackItem.GetArray()) {
			return errors.ErrOverMaxArraySize
		}
	}

	return nil
}

func validatorSetItem(e *ExecutionEngine) error {
	if EvaluationStackCount(e) < 3 {
		return errors.ErrUnderStackLen
	}
	newItem := PeekN(0, e)
	if newItem == nil {
		return errors.ErrBadValue
	}
	index := PeekNInt(1, e)
	if index < 0 {
		return errors.ErrBadValue
	}
	arrItem := PeekN(2, e)
	if arrItem == nil {
		return errors.ErrBadValue
	}
	item := arrItem.GetStackItem()
	if _, ok := item.(*types.Array); !ok {
		if _, ok := item.(*types.ByteArray); ok {
			l := len(item.GetByteArray())
			if index >= l {
				return errors.ErrOverMaxArraySize
			}
			if len(newItem.GetStackItem().GetByteArray()) == 0 {
				return errors.ErrBadValue
			}
		} else {
			return errors.ErrNotArray
		}
	} else {
		if index >= len(item.GetArray()) {
			return errors.ErrOverMaxArraySize
		}
	}
	return nil
}

func validateNewArray(e *ExecutionEngine) error {
	count := PeekInt(e)
	if uint32(count) > MaxArraySize {
		return errors.ErrOverMaxArraySize
	}
	return nil
}

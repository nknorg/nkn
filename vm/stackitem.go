package vm

import "github.com/nknorg/nkn/vm/types"

type StackItem struct {
	_object types.StackItemInterface
}

func NewStackItem(object types.StackItemInterface) *StackItem {
	var stackItem StackItem
	stackItem._object = object
	return &stackItem
}

func (s *StackItem) GetStackItem() types.StackItemInterface {
	return s._object
}

func (s *StackItem) GetExecutionContext() *ExecutionContext {
	return nil
}

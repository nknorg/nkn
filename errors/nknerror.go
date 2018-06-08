package errors

type nknError struct {
	errmsg    string
	callstack *CallStack
	root      error
	code      ErrCode
}

func (e nknError) Error() string {
	return e.errmsg
}

func (e nknError) GetErrCode() ErrCode {
	return e.code
}

func (e nknError) GetRoot() error {
	return e.root
}

func (e nknError) GetCallStack() *CallStack {
	return e.callstack
}

package errors

import (
	"errors"
)

const callStackDepth = 10

type DetailError interface {
	error
	ErrCoder
	CallStacker
	GetRoot()  error
}


func  NewErr(errmsg string) error {
	return errors.New(errmsg)
}

func NewDetailErr(err error,errcode ErrCode,errmsg string) DetailError{
	if err == nil {return nil}

	e, ok := err.(nknError)
	if !ok {
		e.root = err
		e.errmsg = err.Error()
		e.callstack = getCallStack(0, callStackDepth)
		e.code = errcode

	}
	if errmsg != "" {
		e.errmsg = errmsg + ": " + e.errmsg
	}


	return e
}

func RootErr(err error) error {
	if err, ok := err.(DetailError); ok {
		return err.GetRoot()
	}
	return err
}




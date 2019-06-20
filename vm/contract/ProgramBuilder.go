package contract

import (
	"bytes"
	"math/big"

	. "github.com/nknorg/nkn/common"
)

type ProgramBuilder struct {
	buffer bytes.Buffer
}

func NewProgramBuilder() *ProgramBuilder {
	return &ProgramBuilder{}
}

func (pb *ProgramBuilder) AddOp(op OpCode) {
	pb.buffer.WriteByte(byte(op))
}

func (pb *ProgramBuilder) AddCodes(codes []byte) {
	pb.buffer.Write(codes)
}

func (pb *ProgramBuilder) PushNumber(number *big.Int) {
	if number.Cmp(big.NewInt(-1)) == 0 {
		pb.AddOp(PUSHM1)
		return
	}
	if number.Cmp(big.NewInt(0)) == 0 {
		pb.AddOp(PUSH0)
		return
	}
	if number.Cmp(big.NewInt(0)) == 1 && number.Cmp(big.NewInt(16)) <= 0 {
		pb.AddOp(OpCode(byte(PUSH1) - 1 + number.Bytes()[0]))
		return
	}
	pb.PushData(number.Bytes())
}

func (pb *ProgramBuilder) PushData(data []byte) {
	if data == nil {
		return //TODO: add error
	}

	if len(data) <= int(PUSHBYTES75) {
		pb.buffer.WriteByte(byte(len(data)))
		pb.buffer.Write(data[0:len(data)])
	} else if len(data) < 0x100 {
		pb.AddOp(PUSHDATA1)
		pb.buffer.WriteByte(byte(len(data)))
		pb.buffer.Write(data[0:len(data)])
	} else if len(data) < 0x10000 {
		pb.AddOp(PUSHDATA2)
		dataByte := IntToBytes(len(data))
		pb.buffer.Write(dataByte[0:2])
		pb.buffer.Write(data[0:len(data)])
	} else {
		pb.AddOp(PUSHDATA4)
		dataByte := IntToBytes(len(data))
		pb.buffer.Write(dataByte[0:4])
		pb.buffer.Write(data[0:len(data)])
	}
}

func (pb *ProgramBuilder) ToArray() []byte {
	return pb.buffer.Bytes()
}

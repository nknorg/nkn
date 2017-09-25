package evm


type (
	executionFunc  func(evm *ExecutionEngine) ([]byte, error)
	stackValidationFunc func(*Stack) error
)

type OpExec struct {
	Name  string
	Exec  executionFunc
	validateStack stackValidationFunc
	jumps bool
	halts bool
}

func NewOpExecList() [256]OpExec {
	return [256]OpExec{
		STOP: {
			Name:"STOP",
			Exec: opStop,
			validateStack: makeStackFunc(0, 0),
			halts: true,
		},
		ADD: {
			Name: "ADD",
			Exec: opAdd,
			validateStack: makeStackFunc(2, 1),
		},
		MUL: {
			Name: "MUL",
			Exec: opMul,
			validateStack: makeStackFunc(2, 1),
		},
		SUB: {
			Name: "SUB",
			Exec: opSub,
			validateStack: makeStackFunc(2, 1),
		},
		DIV: {
			Name: "DIV",
			Exec: opDiv,
			validateStack: makeStackFunc(2, 1),
		},
		SDIV:{
			Name: "SDIV",
			Exec: opSdiv,
			validateStack: makeStackFunc(2, 1),
		},
		MOD: {
			Name: "MOD",
			Exec: opMod,
			validateStack: makeStackFunc(2, 1),
		},
		SMOD:{
			Name: "SMOD",
			Exec: opSmod,
			validateStack: makeStackFunc(2, 1),
		},
		ADDMOD: {
			Name: "ADDMOD",
			Exec: opAddMod,
			validateStack: makeStackFunc(3, 1),
		},
		MULMOD: {
			Name: "MULMOD",
			Exec: opMulMod,
			validateStack: makeStackFunc(3, 1),
		},
		EXP: {
			Name: "EXP",
			Exec: opExp,
			validateStack: makeStackFunc(2, 1),
		},
		SIGNEXTEND: {
			Name: "SIGNEXTEND",
			Exec: opSignExtend,
			validateStack: makeStackFunc(2, 1),
		},
		LT: {
			Name: "LT",
			Exec: opLt,
			validateStack: makeStackFunc(2, 1),
		},
		GT: {
			Name: "GT",
			Exec: opGt,
			validateStack: makeStackFunc(2, 1),
		},
		SLT: {
			Name: "SLT",
			Exec: opSlt,
			validateStack: makeStackFunc(2, 1),
		},
		SGT: {
			Name: "SGT",
			Exec: opSgt,
			validateStack: makeStackFunc(2, 1),
		},
		EQ: {
			Name: "EQ",
			Exec: opEq,
			validateStack: makeStackFunc(2, 1),
		},
		ISZERO: {
			Name: "ISZERO",
			Exec: opIsZero,
			validateStack: makeStackFunc(1, 1),
		},
		AND: {
			Name: "AND",
			Exec: opAnd,
			validateStack: makeStackFunc(2, 1),
		},
		XOR: {
			Name: "XOR",
			Exec: opXor,
			validateStack: makeStackFunc(2, 1),
		},
		OR: {
			Name: "OR",
			Exec: opOr,
			validateStack: makeStackFunc(2, 1),
		},
		NOT: {
			Name: "NOT",
			Exec: opNot,
			validateStack: makeStackFunc(1, 1),
		},
		BYTE: {
			Name: "BYTE",
			Exec: opByte,
			validateStack: makeStackFunc(2, 1),
		},
		SHA3: {
			Name: "SHA3",
			Exec: opSha3,
			validateStack: makeStackFunc(2, 1),
		},
		ADDRESS: {
			Name: "ADDRESS",
			Exec: opAddress,
			validateStack: makeStackFunc(0, 1),
		},
		BALANCE: {
			Name: "BALANCE",
			Exec: opBalance,
			validateStack: makeStackFunc(1, 1),
		},
		ORIGIN: {
			Name: "ORIGIN",
			Exec: opOrigin,
			validateStack: makeStackFunc(0, 1),
		},
		CALLER: {
			Name: "CALLER",
			Exec: opCaller,
			validateStack: makeStackFunc(0, 1),
		},
		CALLVALUE: {
			Name: "CALLVALUE",
			Exec: opCallValue,
			validateStack: makeStackFunc(0, 1),
		},
		CALLDATALOAD: {
			Name: "CALLDATALOAD",
			Exec: opCallDataLoad,
			validateStack: makeStackFunc(1, 1),
		},
		CALLDATASIZE: {
			Name: "CALLDATASIZE",
			Exec: opCallDataSize,
			validateStack: makeStackFunc(0, 1),
		},
		CALLDATACOPY: {
			Name: "CALLDATACOPY",
			Exec: opCallDataCopy,
			validateStack: makeStackFunc(3, 0),
		},
		CODESIZE: {
			Name: "CODESIZE",
			Exec: opCodeSize,
			validateStack: makeStackFunc(0, 1),
		},
		CODECOPY: {
			Name: "CODECOPY",
			Exec: opCodeCopy,
			validateStack: makeStackFunc(3, 0),
		},
		GASPRICE: {
			Name: "GASPRICE",
			Exec: opGasPrice,
			validateStack: makeStackFunc(0, 1),
		},
		EXTCODESIZE: {
			Name: "EXTCODESIZE",
			Exec: opExtCodeSize,
			validateStack: makeStackFunc(1, 1),
		},
		EXTCODECOPY: {
			Name: "EXTCODECOPY",
			Exec: opExtCodeCopy,
			validateStack: makeStackFunc(4, 0),
		},
		BLOCKHASH: {
			Name: "BLOCKHASH",
			Exec: opBlockHash,
			validateStack: makeStackFunc(1, 1),
		},
		COINBASE: {
			Name: "COINBASE",
			Exec: opCoinBase,
			validateStack: makeStackFunc(0, 1),
		},
		TIMESTAMP: {
			Name: "TIMESTAMP",
			Exec: opTimeStamp,
			validateStack: makeStackFunc(0, 1),
		},
		NUMBER: {
			Name: "NUMBER",
			Exec: opNumber,
			validateStack: makeStackFunc(0, 1),
		},
		DIFFICULTY: {
			Name: "DIFFICULTY",
			Exec: opDifficulty,
			validateStack: makeStackFunc(0, 1),
		},
		GASLIMIT: {
			Name: "GASLIMIT",
			Exec: opGasLimit,
			validateStack: makeStackFunc(0, 1),
		},
		POP: {
			Name: "POP",
			Exec: opPop,
			validateStack: makeStackFunc(1, 0),
		},
		MLOAD: {
			Name: "MLOAD",
			Exec: opMload,
			validateStack: makeStackFunc(1, 1),
		},
		MSTORE: {
			Name: "MSTORE",
			Exec: opMstore,
			validateStack: makeStackFunc(2, 0),
		},
		MSTORE8: {
			Name: "MSTORE8",
			Exec: opMstore8,
			validateStack: makeStackFunc(2, 0),
		},
		SLOAD: {
			Name: "SLOAD",
			Exec: opSload,
			validateStack: makeStackFunc(1, 1),
		},
		SSTORE: {
			Name: "SSTORE",
			Exec: opSstore,
			validateStack: makeStackFunc(2, 0),
		},
		JUMP: {
			Name: "JUMP",
			Exec: opJump,
			validateStack: makeStackFunc(1, 0),
			jumps: true,
		},
		JUMPI: {
			Name: "JUMPI",
			Exec: opJumpPi,
			validateStack: makeStackFunc(2, 0),
			jumps: true,
		},
		PC: {
			Name: "PC",
			Exec: opPc,
			validateStack: makeStackFunc(0, 1),
		},
		MSIZE: {
			Name: "MSIZE",
			Exec: opMsize,
			validateStack: makeStackFunc(0, 1),
		},
		GAS: {
			Name: "GAS",
			Exec: opGas,
			validateStack: makeStackFunc(0, 1),
		},
		JUMPDEST: {
			Name: "JUMPDEST",
			Exec: opJumpDest,
			validateStack: makeStackFunc(0, 0),
		},
		PUSH1: {
			Name: "PUSH1",
			Exec: makePush(1, 1),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH2: {
			Name: "PUSH2",
			Exec: makePush(2, 2),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH3: {
			Name: "PUSH3",
			Exec: makePush(3, 3),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH4: {
			Name: "PUSH4",
			Exec: makePush(4, 4),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH5: {
			Name: "PUSH5",
			Exec: makePush(5, 5),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH6: {
			Name: "PUSH6",
			Exec: makePush(6, 6),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH7: {
			Name: "PUSH7",
			Exec: makePush(7, 7),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH8: {
			Name: "PUSH8",
			Exec: makePush(8, 8),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH9: {
			Name: "PUSH9",
			Exec: makePush(9, 9),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH10: {
			Name: "PUSH10",
			Exec: makePush(10, 10),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH11: {
			Name: "PUSH11",
			Exec: makePush(11, 11),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH12: {
			Name: "PUSH12",
			Exec: makePush(12, 12),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH13: {
			Name: "PUSH13",
			Exec: makePush(13, 13),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH14: {
			Name: "PUSH14",
			Exec: makePush(14, 14),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH15: {
			Name: "PUSH15",
			Exec: makePush(15, 15),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH16: {
			Name: "PUSH16",
			Exec: makePush(16, 16),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH17: {
			Name: "PUSH17",
			Exec: makePush(17, 17),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH18: {
			Name: "PUSH18",
			Exec: makePush(18, 18),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH19: {
			Name:  "PUSH19",
			Exec: makePush(19, 19),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH20: {
			Name: "PUSH20",
			Exec: makePush(20, 20),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH21: {
			Name: "PUSH21",
			Exec: makePush(21, 21),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH22: {
			Name: "PUSH22",
			Exec: makePush(22, 22),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH23: {
			Name: "PUSH23",
			Exec: makePush(23, 23),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH24: {
			Name: "PUSH24",
			Exec: makePush(24, 24),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH25: {
			Name: "PUSH25",
			Exec: makePush(25, 25),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH26: {
			Name: "PUSH26",
			Exec: makePush(26, 26),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH27: {
			Name: "PUSH27",
			Exec: makePush(27, 27),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH28: {
			Name: "PUSH28",
			Exec: makePush(28, 28),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH29: {
			Name: "PUSH29",
			Exec: makePush(29, 29),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH30: {
			Name: "PUSH30",
			Exec: makePush(30, 30),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH31: {
			Name: "PUSH31",
			Exec: makePush(31, 31),
			validateStack: makeStackFunc(0, 1),
		},
		PUSH32: {
			Name: "PUSH32",
			Exec: makePush(32, 32),
			validateStack: makeStackFunc(0, 1),
		},
		DUP1: {
			Name:  "DUP1",
			Exec: makeDup(1),
			validateStack: makeDupStackFunc(1),
		},
		DUP2: {
			Name: "DUP2",
			Exec: makeDup(2),
			validateStack: makeDupStackFunc(2),
		},
		DUP3: {
			Name: "DUP3",
			Exec: makeDup(3),
			validateStack: makeDupStackFunc(3),
		},
		DUP4: {
			Name: "DUP4",
			Exec: makeDup(4),
			validateStack: makeDupStackFunc(4),
		},
		DUP5: {
			Name: "DUP5",
			Exec: makeDup(5),
			validateStack: makeDupStackFunc(5),
		},
		DUP6: {
			Name: "DUP6",
			Exec: makeDup(6),
			validateStack: makeDupStackFunc(6),
		},
		DUP7: {
			Name: "DUP7",
			Exec: makeDup(7),
			validateStack: makeDupStackFunc(7),
		},
		DUP8: {
			Name: "DUP8",
			Exec: makeDup(8),
			validateStack: makeDupStackFunc(8),
		},
		DUP9: {
			Name: "DUP9",
			Exec: makeDup(9),
			validateStack: makeDupStackFunc(9),
		},
		DUP10: {
			Name: "DUP10",
			Exec: makeDup(10),
			validateStack: makeDupStackFunc(10),
		},
		DUP11: {
			Name: "DUP11",
			Exec: makeDup(11),
			validateStack: makeDupStackFunc(11),
		},
		DUP12: {
			Name: "DUP12",
			Exec: makeDup(12),
			validateStack: makeDupStackFunc(12),
		},
		DUP13: {
			Name: "DUP13",
			Exec: makeDup(13),
			validateStack: makeDupStackFunc(13),
		},
		DUP14: {
			Name: "DUP14",
			Exec: makeDup(14),
			validateStack: makeDupStackFunc(14),
		},
		DUP15: {
			Name: "DUP15",
			Exec: makeDup(15),
			validateStack: makeDupStackFunc(15),
		},
		DUP16: {
			Name: "DUP16",
			Exec: makeDup(16),
			validateStack: makeDupStackFunc(16),
		},
		SWAP1: {
			Name: "SWAP1",
			Exec: makeSwap(1),
			validateStack: makeSwapStackFunc(2),
		},
		SWAP2: {
			Name: "SWAP2",
			Exec: makeSwap(2),
			validateStack: makeSwapStackFunc(3),
		},
		SWAP3: {
			Name: "SWAP3",
			Exec: makeSwap(3),
			validateStack: makeSwapStackFunc(4),
		},
		SWAP4: {
			Name: "SWAP4",
			Exec: makeSwap(4),
			validateStack: makeSwapStackFunc(5),
		},
		SWAP5: {
			Name: "SWAP5",
			Exec: makeSwap(5),
			validateStack: makeSwapStackFunc(6),
		},
		SWAP6: {
			Name: "SWAP6",
			Exec: makeSwap(6),
			validateStack: makeSwapStackFunc(7),
		},
		SWAP7: {
			Name: "SWAP7",
			Exec: makeSwap(7),
			validateStack: makeSwapStackFunc(8),
		},
		SWAP8: {
			Name: "SWAP8",
			Exec: makeSwap(8),
			validateStack: makeSwapStackFunc(9),
		},
		SWAP9: {
			Name: "SWAP9",
			Exec: makeSwap(9),
			validateStack: makeSwapStackFunc(10),
		},
		SWAP10: {
			Name: "SWAP10",
			Exec: makeSwap(10),
			validateStack: makeSwapStackFunc(11),
		},
		SWAP11: {
			Name: "SWAP11",
			Exec: makeSwap(11),
			validateStack: makeSwapStackFunc(12),
		},
		SWAP12: {
			Name: "SWAP12",
			Exec: makeSwap(12),
			validateStack: makeSwapStackFunc(13),
		},
		SWAP13: {
			Name: "SWAP13",
			Exec: makeSwap(13),
			validateStack: makeSwapStackFunc(14),
		},
		SWAP14: {
			Name: "SWAP14",
			Exec: makeSwap(14),
			validateStack: makeSwapStackFunc(15),
		},
		SWAP15: {
			Name: "SWAP15",
			Exec: makeSwap(15),
			validateStack: makeSwapStackFunc(15),
		},
		SWAP16: {
			Name: "SWAP16",
			Exec: makeSwap(16),
			validateStack: makeSwapStackFunc(16),
		},
		LOG0: {
			Name: "LOG0",
			Exec: makeLog(0),
			validateStack: makeStackFunc(2, 0),
		},
		LOG1: {
			Name: "LOG1",
			Exec: makeLog(1),
			validateStack: makeStackFunc(3, 0),
		},
		LOG2: {
			Name: "LOG2",
			Exec: makeLog(2),
			validateStack: makeStackFunc(4, 0),
		},
		LOG3: {
			Name: "LOG3",
			Exec: makeLog(3),
			validateStack: makeStackFunc(5, 0),
		},
		LOG4: {
			Name: "LOG4",
			Exec: makeLog(4),
			validateStack: makeStackFunc(6, 0),
		},
		CREATE: {
			Name: "CREATE",
			Exec: opCreate,
			validateStack: makeStackFunc(3, 1),
		},
		CALL: {
			Name: "CALL",
			Exec: opCall,
			validateStack: makeStackFunc(7, 1),
		},
		CALLCODE: {
			Name: "CALLCODE",
			Exec: opCallCode,
			validateStack: makeStackFunc(7, 1),
		},
		RETURN: {
			Name: "RETURN",
			Exec: opReturn,
			validateStack: makeStackFunc(2, 0),
			halts: true,
		},
		SELFDESTRUCT: {
			Name: "SELFDESTRUCT",
			Exec: opSuicide,
			validateStack: makeStackFunc(1, 0),
			halts: true,
		},
	}
}


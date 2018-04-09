package vm

type OpExec struct {
	Opcode    OpCode
	Name      string
	Exec      func(*ExecutionEngine) (VMState, error)
	Validator func(*ExecutionEngine) error
}

var (
	OpExecList = [256]OpExec{
		// control flow
		PUSH0:       {Opcode: PUSH0, Name: "PUSH0", Exec: opPushData,},
		PUSHBYTES1:  {Opcode: PUSHBYTES1, Name: "PUSHBYTES1", Exec: opPushData},
		PUSHBYTES75: {Opcode: PUSHBYTES75, Name: "PUSHBYTES75", Exec: opPushData},
		PUSHDATA1:   {Opcode: PUSHDATA1, Name: "PUSHDATA1", Exec: opPushData},
		PUSHDATA2:   {Opcode: PUSHDATA2, Name: "PUSHDATA2", Exec: opPushData},
		PUSHDATA4:   {Opcode: PUSHDATA4, Name: "PUSHDATA4", Exec: opPushData, Validator: validatorPushData4},
		PUSHM1:      {Opcode: PUSHM1, Name: "PUSHM1", Exec: opPushData},
		PUSH1:       {Opcode: PUSH1, Name: "PUSH1", Exec: opPushData},
		PUSH2:       {Opcode: PUSH2, Name: "PUSH2", Exec: opPushData},
		PUSH3:       {Opcode: PUSH3, Name: "PUSH3", Exec: opPushData},
		PUSH4:       {Opcode: PUSH4, Name: "PUSH4", Exec: opPushData},
		PUSH5:       {Opcode: PUSH5, Name: "PUSH5", Exec: opPushData},
		PUSH6:       {Opcode: PUSH6, Name: "PUSH6", Exec: opPushData},
		PUSH7:       {Opcode: PUSH7, Name: "PUSH7", Exec: opPushData},
		PUSH8:       {Opcode: PUSH8, Name: "PUSH8", Exec: opPushData},
		PUSH9:       {Opcode: PUSH9, Name: "PUSH9", Exec: opPushData},
		PUSH10:      {Opcode: PUSH10, Name: "PUSH10", Exec: opPushData},
		PUSH11:      {Opcode: PUSH11, Name: "PUSH11", Exec: opPushData},
		PUSH12:      {Opcode: PUSH12, Name: "PUSH12", Exec: opPushData},
		PUSH13:      {Opcode: PUSH13, Name: "PUSH13", Exec: opPushData},
		PUSH14:      {Opcode: PUSH14, Name: "PUSH14", Exec: opPushData},
		PUSH15:      {Opcode: PUSH15, Name: "PUSH15", Exec: opPushData},
		PUSH16:      {Opcode: PUSH16, Name: "PUSH16", Exec: opPushData},

		//Control
		NOP:      {Opcode: NOP, Name: "NOP", Exec: opNop},
		JMP:      {Opcode: JMP, Name: "JMP", Exec: opJmp},
		JMPIF:    {Opcode: JMPIF, Name: "JMPIF", Exec: opJmp},
		JMPIFNOT: {Opcode: JMPIFNOT, Name: "JMPIFNOT", Exec: opJmp},
		CALL:     {Opcode: CALL, Name: "CALL", Exec: opCall, Validator: validateCall},
		RET:      {Opcode: RET, Name: "RET", Exec: opRet},
		APPCALL:  {Opcode: APPCALL, Name: "APPCALL", Exec: opAppCall, Validator: validateAppCall},
		TAILCALL: {Opcode: TAILCALL, Name: "TAILCALL", Exec: opAppCall},
		SYSCALL:  {Opcode: SYSCALL, Name: "SYSCALL", Exec: opSysCall, Validator: validateSysCall},

		//Stack ops
		TOALTSTACK:   {Opcode: TOALTSTACK, Name: "TOALTSTACK", Exec: opToAltStack},
		FROMALTSTACK: {Opcode: FROMALTSTACK, Name: "FROMALTSTACK", Exec: opFromAltStack},
		XDROP:        {Opcode: XDROP, Name: "XDROP", Exec: opXDrop, Validator: validateXDrop},
		XSWAP:        {Opcode: XSWAP, Name: "XSWAP", Exec: opXSwap, Validator: validateXSwap},
		XTUCK:        {Opcode: XTUCK, Name: "XTUCK", Exec: opXTuck, Validator: validateXTuck},
		DEPTH:        {Opcode: DEPTH, Name: "DEPTH", Exec: opDepth},
		DROP:         {Opcode: DROP, Name: "DROP", Exec: opDrop},
		DUP:          {Opcode: DUP, Name: "DUP", Exec: opDup},
		NIP:          {Opcode: NIP, Name: "NIP", Exec: opNip},
		OVER:         {Opcode: OVER, Name: "OVER", Exec: opOver},
		PICK:         {Opcode: PICK, Name: "PICK", Exec: opPick, Validator: validatePick},
		ROLL:         {Opcode: ROLL, Name: "ROLL", Exec: opRoll, Validator: validateRoll},
		ROT:          {Opcode: ROT, Name: "ROT", Exec: opRot},
		SWAP:         {Opcode: SWAP, Name: "SWAP", Exec: opSwap},
		TUCK:         {Opcode: TUCK, Name: "TUCK", Exec: opTuck},

		//Splice
		CAT:    {Opcode: CAT, Name: "CAT", Exec: opCat, Validator: validateCat},
		SUBSTR: {Opcode: SUBSTR, Name: "SUBSTR", Exec: opSubStr, Validator: validateSubStr},
		LEFT:   {Opcode: LEFT, Name: "LEFT", Exec: opLeft, Validator: validateLeft},
		RIGHT:  {Opcode: RIGHT, Name: "RIGHT", Exec: opRight, Validator: validateRight},
		SIZE:   {Opcode: SIZE, Name: "SIZE", Exec: opSize},

		//Bitwiase logic
		INVERT: {Opcode: INVERT, Name: "INVERT", Exec: opInvert},
		AND:    {Opcode: AND, Name: "AND", Exec: opBigIntZip},
		OR:     {Opcode: OR, Name: "OR", Exec: opBigIntZip},
		XOR:    {Opcode: XOR, Name: "XOR", Exec: opBigIntZip},
		EQUAL:  {Opcode: EQUAL, Name: "EQUAL", Exec: opEqual},

		//Arithmetic
		INC:         {Opcode: INC, Name: "INC", Exec: opBigInt},
		DEC:         {Opcode: DEC, Name: "DEC", Exec: opBigInt},
		NEGATE:      {Opcode: NEGATE, Name: "NEGATE", Exec: opBigInt},
		ABS:         {Opcode: ABS, Name: "ABS", Exec: opBigInt},
		NOT:         {Opcode: NOT, Name: "NOT", Exec: opNot},
		NZ:          {Opcode: NZ, Name: "NZ", Exec: opNz},
		ADD:         {Opcode: ADD, Name: "ADD", Exec: opBigIntZip},
		SUB:         {Opcode: SUB, Name: "SUB", Exec: opBigIntZip},
		MUL:         {Opcode: MUL, Name: "MUL", Exec: opBigIntZip},
		DIV:         {Opcode: DIV, Name: "DIV", Exec: opBigIntZip},
		MOD:         {Opcode: MOD, Name: "MOD", Exec: opBigIntZip},
		SHL:         {Opcode: SHL, Name: "SHL", Exec: opBigIntZip},
		SHR:         {Opcode: SHR, Name: "SHR", Exec: opBigIntZip},
		BOOLAND:     {Opcode: BOOLAND, Name: "BOOLAND", Exec: opBoolZip},
		BOOLOR:      {Opcode: BOOLOR, Name: "BOOLOR", Exec: opBoolZip},
		NUMEQUAL:    {Opcode: NUMEQUAL, Name: "NUMEQUAL", Exec: opBigIntComp, Validator: validatorBigIntComp},
		NUMNOTEQUAL: {Opcode: NUMNOTEQUAL, Name: "NUMNOTEQUAL", Exec: opBigIntComp, Validator: validatorBigIntComp},
		LT:          {Opcode: LT, Name: "LT", Exec: opBigIntComp},
		GT:          {Opcode: GT, Name: "GT", Exec: opBigIntComp},
		LTE:         {Opcode: LTE, Name: "LTE", Exec: opBigIntComp},
		GTE:         {Opcode: GTE, Name: "GTE", Exec: opBigIntComp},
		MIN:         {Opcode: MIN, Name: "MIN", Exec: opBigIntZip},
		MAX:         {Opcode: MAX, Name: "MAX", Exec: opBigIntZip},
		WITHIN:      {Opcode: WITHIN, Name: "WITHIN", Exec: opWithIn},

		//Crypto
		SHA1:          {Opcode: SHA1, Name: "SHA1", Exec: opHash},
		SHA256:        {Opcode: SHA256, Name: "SHA256", Exec: opHash},
		HASH160:       {Opcode: HASH160, Name: "HASH160", Exec: opHash},
		HASH256:       {Opcode: HASH256, Name: "HASH256", Exec: opHash},
		CHECKSIG:      {Opcode: CHECKSIG, Name: "CHECKSIG", Exec: opCheckSig},
		CHECKMULTISIG: {Opcode: CHECKMULTISIG, Name: "CHECKMULTISIG", Exec: opCheckMultiSig},

		//Array
		ARRAYSIZE: {Opcode: ARRAYSIZE, Name: "ARRAYSIZE", Exec: opArraySize},
		PACK:      {Opcode: PACK, Name: "PACK", Exec: opPack, Validator: validatePack},
		UNPACK:    {Opcode: UNPACK, Name: "UNPACK", Exec: opUnpack},
		PICKITEM:  {Opcode: PICKITEM, Name: "PICKITEM", Exec: opPickItem, Validator: validatePickItem},
		SETITEM:   {Opcode: SETITEM, Name: "SETITEM", Exec: opSetItem, Validator: validatorSetItem},
		NEWARRAY:  {Opcode: NEWARRAY, Name: "NEWARRAY", Exec: opNewArray, Validator: validateNewArray},
	}
)

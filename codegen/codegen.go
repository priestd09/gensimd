package codegen

import (
	"errors"
	"fmt"
	"go/token"
	"math"
	"strconv"
	"strings"

	"golang.org/x/tools/go/types"

	"reflect"

	"github.com/bjwbell/gensimd/simd"

	"golang.org/x/tools/go/ssa"
)

type phiInfo struct {
	value ssa.Value
	phi   *ssa.Phi
}

type Function struct {
	// output function name
	outfn     string
	Indent    string
	ssa       *ssa.Function
	registers map[string]bool // maps register to false if unused and true if used
	ssaNames  map[string]nameInfo
	// map from block index to the successor block indexes that need phi vars set
	phiInfo map[int]map[int][]phiInfo
}

type nameInfo struct {
	name  string
	typ   types.Type
	local *varInfo
	param *paramInfo
}

// RegAndOffset returns the register and offset to access the nameInfo memory.
// For locals the register is the stack pointer (SP) and for params the register
// is the frame pointer (FP).
func (name *nameInfo) MemRegOffsetSize() (reg register, offset uint, size uint) {
	if name.local != nil {
		reg = *getRegister(REG_SP)
		offset = name.local.offset
		size = name.local.size
	} else if name.param != nil {
		reg = *getRegister(REG_FP)
		offset = name.param.offset
		size = name.param.size
	} else {
		panic(fmt.Sprintf("nameInfo (%v) is not a local or param", name))
	}
	return
}

func (name *nameInfo) IsSsaLocal() bool {
	return name.local != nil && name.local.info != nil
}

func (name *nameInfo) IsPointer() bool {
	_, ok := name.typ.(*types.Pointer)
	return ok
}

func (name *nameInfo) PointerUnderlyingType() types.Type {
	if !name.IsPointer() {
		panic(fmt.Sprintf("nameInfo (%v) not pointer type in PointerUnderlyingType", name))
	}
	ptrType := name.typ.(*types.Pointer)
	return ptrType.Elem()
}

func (name *nameInfo) IsArray() bool {
	_, ok := name.typ.(*types.Array)
	return ok
}

func (name *nameInfo) IsSlice() bool {
	_, ok := name.typ.(*types.Slice)
	return ok
}

func (name *nameInfo) IsBasic() bool {
	_, ok := name.typ.(*types.Basic)
	return ok
}

func (name *nameInfo) IsInteger() bool {
	if !name.IsBasic() {
		return false
	}
	t := name.typ.(*types.Basic)
	return t.Info()&types.IsInteger == types.IsInteger
}

type varInfo struct {
	name string
	// offset from the stack pointer (SP)
	offset uint
	size   uint
	info   *ssa.Alloc
}

func (v *varInfo) ssaName() string {
	return v.info.Name()
}

type paramInfo struct {
	name string
	// offset from the frame pointer (FP)
	offset uint
	size   uint
	info   *ssa.Parameter
	extra  interface{}
}

func (p *paramInfo) ssaName() string {
	return p.info.Name()
}

type paramSlice struct {
	lenOffset uint
}

type Error struct {
	Err error
	Pos token.Pos
}

func CreateFunction(fn *ssa.Function, outfn string) (*Function, *Error) {
	if fn == nil {
		return nil, &Error{Err: errors.New("Nil function passed in")}
	}
	f := Function{ssa: fn, outfn: outfn}
	f.Indent = "        "
	f.init()
	return &f, nil
}

func (f *Function) GoAssembly() (string, *Error) {
	return f.asmFunc()
}

func memFn(name string, offset uint, regName string) func() string {
	return func() string {
		return fmt.Sprintf("%v+%v(%v)", name, offset, regName)
	}
}

func regFn(name string) func() string {
	return func() string {
		return name
	}
}

func (f *Function) asmParams() (string, *Error) {
	// offset in bytes from frame pointer (FP)
	offset := uint(0)
	asm := ""
	for _, p := range f.ssa.Params {
		param := paramInfo{name: p.Name(), offset: offset, info: p, size: sizeof(p.Type())}
		// TODO alloc reg based on other param types
		if _, ok := p.Type().(*types.Slice); ok {
			param.extra = paramSlice{lenOffset: offset + pointerSize}
		} else if basic, ok := p.Type().(*types.Basic); ok && basic.Kind() == types.Int {
		} else {
			return "", &Error{Err: errors.New("Unsupported param type"), Pos: p.Pos()}
		}
		f.ssaNames[param.name] = nameInfo{name: param.name, typ: param.info.Type(),
			local: nil, param: &param}
		offset += param.size
	}
	return asm, nil
}

func (f *Function) asmFunc() (string, *Error) {

	params, err := f.asmParams()
	if err != nil {
		return params, err
	}

	zeroRetValue, err := f.asmZeroRetValue()
	if err != nil {
		return params + zeroRetValue, err
	}

	zeroSsaLocals, err := f.asmZeroSsaLocals()
	if err != nil {
		return params + zeroRetValue + zeroSsaLocals, err
	}

	if err := f.computePhi(); err != nil {
		return "", err
	}

	basicblocks, err := f.asmBasicBlocks()
	if err != nil {
		return params + zeroRetValue + zeroSsaLocals + basicblocks, err
	}

	zeroNonSsaLocals, err := f.asmZeroNonSsaLocals()
	if err != nil {
		return zeroNonSsaLocals, err
	}

	frameSize := f.localsSize()
	asm := params
	asm += f.asmSetStackPointer()
	asm += zeroRetValue
	asm += zeroSsaLocals
	asm += zeroNonSsaLocals
	asm += basicblocks
	asm = f.fixupRets(asm)
	a := fmt.Sprintf("TEXT ·%v(SB),NOSPLIT,$%v-%v\n%v", f.outfname(), frameSize, f.paramsSize()+f.retSize(), asm)
	return a, nil
}

func (f *Function) GoProto() string {
	pkgname := "package " + f.ssa.Package().Pkg.Name() + "\n"
	fnproto := "func " + f.outfname() + "(" + strings.TrimPrefix(f.ssa.Signature.String(), "func(")
	proto := pkgname + "\n"
	proto += fnproto + "\n"
	return proto
}

func (f *Function) outfname() string {
	if f.outfn != "" {
		return f.outfn
	}
	return f.ssa.Name()
}

func (f *Function) asmZeroSsaLocals() (string, *Error) {
	asm := ""
	offset := uint(0)
	locals := f.ssa.Locals
	for _, local := range locals {
		if local.Heap {

			msg := fmt.Errorf("Can't heap alloc local, name: %v", local.Name())
			return "", &Error{Err: msg, Pos: local.Pos()}
		}
		sp := getRegister(REG_SP)

		//local values are always addresses, and have pointer types, so the type
		//of the allocated variable is actually
		//Type().Underlying().(*types.Pointer).Elem().
		typ := local.Type().Underlying().(*types.Pointer).Elem()
		size := sizeof(typ)
		asm += asmZeroMemory(f.Indent, local.Name(), offset, size, sp)
		v := varInfo{name: local.Name(), offset: offset, size: size, info: local}
		f.ssaNames[v.name] = nameInfo{name: v.name, typ: typ, local: &v, param: nil}

		offset += size
	}
	return asm, nil
}

func (f *Function) asmAllocLocal(name string, typ types.Type) (nameInfo, *Error) {
	size := sizeof(typ)
	//single byte size not supported
	if size == 1 {
		size = 8
	}
	v := varInfo{name: name, offset: uint(f.localsSize()), size: size, info: nil}
	info := nameInfo{name: name, typ: typ, param: nil, local: &v}
	f.ssaNames[v.name] = info
	// zeroing the memory is done at the beginning of the function
	//asmZeroMemory(f.Indent, v.name, v.offset, v.size, sp)
	return info, nil
}

func (f *Function) asmZeroNonSsaLocals() (string, *Error) {
	asm := ""
	for _, name := range f.ssaNames {
		if name.local == nil || name.IsSsaLocal() {
			continue
		}
		sp := getRegister(REG_SP)
		// single byte size is not supported
		if name.local.size == 1 {
			name.local.size = 8
		}
		asm += asmZeroMemory(f.Indent, name.name, name.local.offset, name.local.size, sp)
	}
	return asm, nil
}

func (f *Function) asmZeroRetValue() (string, *Error) {
	asm := asmZeroMemory(f.Indent, retName(), f.retOffset(), f.retSize(), getRegister(REG_FP))
	return asm, nil
}

func (f *Function) asmBasicBlocks() (string, *Error) {
	asm := ""
	for i := 0; i < len(f.ssa.Blocks); i++ {
		a, err := f.asmBasicBlock(f.ssa.Blocks[i])
		asm += a
		if err != nil {
			return asm, err
		}
	}
	return asm, nil
}

func (f *Function) asmBasicBlock(block *ssa.BasicBlock) (string, *Error) {
	asm := "block" + strconv.Itoa(block.Index) + ":\n"
	for i := 0; i < len(block.Instrs); i++ {
		a, err := f.asmInstr(block.Instrs[i])
		asm += a
		if err != nil {
			return asm, err
		}

	}
	return asm, nil
}

func (f *Function) asmInstr(instr ssa.Instruction) (string, *Error) {

	if instr == nil {
		panic("Nil instr in asmInstr")
	}
	asm := ""
	caseAsm := ""
	var caseErr *Error
	switch instr := instr.(type) {
	default:
		caseAsm = f.Indent + fmt.Sprintf("Unknown ssa instruction: %v\n", instr)
	case *ssa.Alloc:
		caseAsm, caseErr = f.asmAllocInstr(instr)
	case *ssa.BinOp:
		caseAsm, caseErr = f.asmBinOp(instr)
	case *ssa.Call:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Call: %v, name: %v\n", instr, instr.Name())
	case *ssa.ChangeInterface:
		caseAsm = f.Indent + fmt.Sprintf("ssa.ChangeInterface: %v, name: %v\n", instr, instr.Name())
	case *ssa.ChangeType:
		caseAsm = f.Indent + fmt.Sprintf("ssa.ChangeType: %v, name: %v\n", instr, instr.Name())
	case *ssa.Convert:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Convert: %v, name: %v\n", instr, instr.Name())
	case *ssa.Defer:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Defer: %v\n", instr)
	case *ssa.Extract:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Extra: %v, name: %v\n", instr, instr.Name())
	case *ssa.Field:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Field: %v, name: %v\n", instr, instr.Name())
	case *ssa.FieldAddr:
		caseAsm = f.Indent + fmt.Sprintf("ssa.FieldAddr: %v, name: %v\n", instr, instr.Name())
	case *ssa.Go:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Go: %v\n", instr)
	case *ssa.If:
		caseAsm, caseErr = f.asmIf(instr)
	case *ssa.Index:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Index: %v, name: %v\n", instr, instr.Name())
	case *ssa.IndexAddr:
		caseAsm, caseErr = f.asmIndexAddr(instr)
	case *ssa.Jump:
		caseAsm, caseErr = f.asmJump(instr)
	case *ssa.Lookup:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Lookup: %v, name: %v\n", instr, instr.Name())
	case *ssa.MakeChan:
		caseAsm = f.Indent + fmt.Sprintf("ssa.MakeChan: %v, name: %v\n", instr, instr.Name())
	case *ssa.MakeClosure:
		caseAsm = f.Indent + fmt.Sprintf("ssa.MakeClosure: %v, name: %v\n", instr, instr.Name())
	case *ssa.MakeInterface:
		caseAsm = f.Indent + fmt.Sprintf("ssa.MakeInterface: %v, name: %v\n", instr, instr.Name())
	case *ssa.MakeMap:
		caseAsm = f.Indent + fmt.Sprintf("ssa.MakeMap: %v, name: %v\n", instr, instr.Name())
	case *ssa.MakeSlice:
		caseAsm = f.Indent + fmt.Sprintf("ssa.MakeSlice: %v, name: %v\n", instr, instr.Name())
	case *ssa.MapUpdate:
		caseAsm = f.Indent + fmt.Sprintf("ssa.MapUpdate: %v\n", instr)
	case *ssa.Next:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Next: %v, name: %v\n", instr, instr.Name())
	case *ssa.Panic:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Panic: %v", instr) + "\n"
	case *ssa.Phi:
		caseAsm, caseErr = f.asmPhi(instr)
	case *ssa.Range:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Range: %v, name: %v\n", instr, instr.Name())
	case *ssa.Return:
		caseAsm, caseErr = f.asmReturn(instr)
	case *ssa.RunDefers:
		caseAsm = f.Indent + fmt.Sprintf("ssa.RunDefers: %v", instr) + "\n"
	case *ssa.Select:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Select: %v, name: %v\n", instr, instr.Name())
	case *ssa.Send:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Send: %v", instr) + "\n"
	case *ssa.Slice:
		caseAsm = f.Indent + fmt.Sprintf("ssa.Slice: %v, name: %v\n", instr, instr.Name())
	case *ssa.Store:
		caseAsm, caseErr = f.asmStore(instr)
	case *ssa.TypeAssert:
		caseAsm = f.Indent + fmt.Sprintf("ssa.TypeAssert: %v, name: %v\n", instr, instr.Name())
	case *ssa.UnOp:
		caseAsm, caseErr = f.asmUnOp(instr)
	}

	if caseErr != nil {
		return caseAsm, caseErr
	} else {
		asm += caseAsm
	}

	return asm, nil
}

func (f *Function) asmIf(instr *ssa.If) (string, *Error) {
	asm := ""
	tblock, fblock := -1, -1
	if instr.Block() != nil && len(instr.Block().Succs) == 2 {
		tblock = instr.Block().Succs[0].Index
		fblock = instr.Block().Succs[1].Index

	}
	if tblock == -1 || fblock == -1 {
		panic("asmIf: malformed CFG")
	}
	if info, ok := f.ssaNames[instr.Cond.Name()]; !ok {
		err := fmt.Errorf("asmIf: unhandled case, cond (%v)", instr.Cond)
		return "", &Error{Err: err, Pos: instr.Pos()}
	} else {
		a, err := f.asmJumpPreamble(instr.Block().Index, fblock)
		if err != nil {
			return "", err
		}
		asm += a
		r, offset, _ := info.MemRegOffsetSize()
		asm += asmCmpMemImm32(f.Indent, info.name, uint32(offset), &r, uint32(0))
		asm += f.Indent + "JEQ    " + "block" + strconv.Itoa(fblock) + "\n"
		a, err = f.asmJumpPreamble(instr.Block().Index, tblock)
		if err != nil {
			return "", err
		}
		asm += a
		asm += f.Indent + "JMP    " + "block" + strconv.Itoa(tblock) + "\n"

	}
	asm = f.Indent + fmt.Sprintf("// BEGIN ssa.If, %v\n", instr) + asm
	asm += f.Indent + fmt.Sprintf("// END ssa.If, %v\n", instr)
	return asm, nil
}

func (f *Function) asmJumpPreamble(blockIndex, jmpIndex int) (string, *Error) {
	asm := ""
	phiInfos := f.phiInfo[blockIndex][jmpIndex]
	for _, phiInfo := range phiInfos {
		store := ssa.Store{Addr: phiInfo.phi, Val: phiInfo.value}
		if a, err := f.asmStore(&store); err != nil {
			return asm, err
		} else {
			asm += a
		}
	}
	return asm, nil
}

func (f *Function) asmJump(jmp *ssa.Jump) (string, *Error) {
	asm := ""
	block := -1
	if jmp.Block() != nil && len(jmp.Block().Succs) == 1 {
		block = jmp.Block().Succs[0].Index
	} else {
		panic("asmJump: malformed CFG")
	}
	a, err := f.asmJumpPreamble(jmp.Block().Index, block)
	if err != nil {
		return "", err
	}
	asm += a
	asm += f.Indent + "JMP block" + strconv.Itoa(block) + "\n"
	asm = f.Indent + "// BEGIN ssa.Jump\n" + asm
	asm += f.Indent + "// END ssa.Jump\n"
	return asm, nil
}

func (f *Function) computePhi() *Error {
	for i := 0; i < len(f.ssa.Blocks); i++ {
		if err := f.computeBasicBlockPhi(f.ssa.Blocks[i]); err != nil {
			return err
		}
	}
	return nil
}

func (f *Function) computeBasicBlockPhi(block *ssa.BasicBlock) *Error {
	for i := 0; i < len(block.Instrs); i++ {
		instr := block.Instrs[i]
		switch instr := instr.(type) {
		default:
			break
		case *ssa.Phi:
			if err := f.computePhiInstr(instr); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *Function) computePhiInstr(phi *ssa.Phi) *Error {
	blockIndex := phi.Block().Index
	for i, edge := range phi.Edges {
		edgeBlock := -1
		if phi.Block() != nil && i < len(phi.Block().Preds) {
			edgeBlock = phi.Block().Preds[i].Index
		}
		if edgeBlock == -1 {
			panic("computePhiInstr: malformed CFG")
		}
		if _, ok := f.phiInfo[edgeBlock]; !ok {
			f.phiInfo[edgeBlock] = make(map[int][]phiInfo)
		}
		f.phiInfo[edgeBlock][blockIndex] = append(f.phiInfo[edgeBlock][blockIndex], phiInfo{value: edge, phi: phi})
	}
	return nil
}

func (f *Function) asmPhi(phi *ssa.Phi) (string, *Error) {
	if err := f.allocValueOnDemand(phi); err != nil {
		return "", err
	}
	asm := f.Indent
	asm += fmt.Sprintf("// BEGIN ssa.Phi, name (%v), comment (%v), value (%v)\n", phi.Name(), phi.Comment, phi)
	asm += f.Indent + fmt.Sprintf("// END ssa.Phi, %v\n", phi)
	return asm, nil
}

var dummySpSize = uint32(math.MaxUint32)

func (f *Function) asmReturn(ret *ssa.Return) (string, *Error) {
	asm := asmResetStackPointer(f.Indent, dummySpSize)
	asm = f.Indent + "// BEGIN ssa.Return\n" + asm
	if a, err := f.asmCopyToRet(ret.Results); err != nil {
		return "", err
	} else {
		asm += a
	}
	asm += asmRet(f.Indent)
	asm += f.Indent + "// END ssa.Return\n"
	return asm, nil
}

func (f *Function) asmCopyToRet(val []ssa.Value) (string, *Error) {
	if len(val) == 0 {
		return "", nil
	}
	if len(val) > 1 {
		err := Error{
			Err: fmt.Errorf("Multiple return values not supported"),
			Pos: 0}
		return "", &err
	}
	retAddr := nameInfo{name: retName(), typ: f.retType(), local: nil, param: f.retParam()}
	return f.asmStoreValAddr(val[0], &retAddr)
}

func asmResetStackPointer(indent string, size uint32) string {
	sp := getRegister(REG_SP)
	return asmAddImm32Reg(indent, size, sp)
}

func (f *Function) fixupRets(asm string) string {
	old := asmResetStackPointer(f.Indent, dummySpSize)
	new := asmResetStackPointer(f.Indent, f.localsSize())
	return strings.Replace(asm, old, new, -1)
}

func (f *Function) asmSetStackPointer() string {
	sp := getRegister(REG_SP)
	asm := asmSubImm32Reg(f.Indent, uint32(f.localsSize()), sp)
	return asm
}

func (f *Function) asmStoreValAddr(val ssa.Value, addr *nameInfo) (string, *Error) {
	var err *Error
	if err = f.allocValueOnDemand(val); err != nil {
		return "", err
	}
	if addr.local == nil && addr.param == nil {
		msg := fmt.Errorf("In asmStoreValAddr invalid addr \"%v\"", addr)
		return "", &Error{Err: msg, Pos: 0}
	}

	asm := ""
	asm += f.Indent + fmt.Sprintf("// BEGIN asmStoreValAddr addr name:%v, val name:%v\n", addr.name, val.Name()) + asm
	size := f.sizeof(val)
	iterations := size / intSize()

	if size > intSize() {
		if size%intSize() != 0 {
			panic(fmt.Sprintf("size (%v) not multiple of intSize (%v) in asmStore", size, intSize()))
		}
	}

	valReg := f.allocReg(DataReg, intSize())

	for i := uint(0); i < iterations; i++ {
		offset := i * intSize()
		a, err := f.asmLoadValue(val, offset, intSize(), &valReg)
		if err != nil {
			return a, err
		}
		asm += a
		a, err = f.asmStoreReg(&valReg, addr, offset)
		if err != nil {
			return a, err
		}
		asm += a
	}

	f.freeReg(valReg)
	asm += f.Indent + fmt.Sprintf("// END asmStoreValAddr addr name:%v, val name:%v\n", addr.name, val.Name())
	return asm, nil
}

func (f *Function) asmStore(instr *ssa.Store) (string, *Error) {
	var err *Error
	if err = f.allocValueOnDemand(instr.Val); err != nil {
		return "", err
	}
	if err = f.allocValueOnDemand(instr.Addr); err != nil {
		return "", err
	}

	asm := ""
	asm += f.Indent + fmt.Sprintf("// BEGIN ssa.Store addr name:%v, val name:%v\n", instr.Addr.Name(), instr.Val.Name()) + asm
	size := f.sizeof(instr.Val)
	iterations := size / intSize()

	if size > intSize() {
		if size%intSize() != 0 {
			panic(fmt.Sprintf("size (%v) not multiple of intSize (%v) in asmStore", size, intSize()))
		}
	}

	valReg := f.allocReg(DataReg, intSize())

	for i := uint(0); i < iterations; i++ {
		offset := i * intSize()
		a, err := f.asmLoadValue(instr.Val, offset, intSize(), &valReg)
		if err != nil {
			return a, err
		}
		asm += a
		addr, ok := f.ssaNames[instr.Addr.Name()]
		if !ok {
			panic(fmt.Sprintf("Unknown name (%v) in asmStore, addr (%v)\n", instr.Addr.Name(), instr.Addr))
		}

		a, err = f.asmStoreReg(&valReg, &addr, offset)
		if err != nil {
			return a, err
		}
		asm += a
	}

	f.freeReg(valReg)
	asm += f.Indent + fmt.Sprintf("// END ssa.Store addr name:%v, val name:%v\n", instr.Addr.Name(), instr.Val.Name())
	return asm, nil
}

func (f *Function) asmBinOp(instr *ssa.BinOp) (string, *Error) {
	if err := f.allocValueOnDemand(instr); err != nil {
		return "", err
	}
	var regX, regY *register
	var regVal register
	// comparison op results are size 1 byte, but that's not supported
	if f.sizeof(instr) == 1 {
		regVal = f.allocReg(DataReg, 8*f.sizeof(instr))
	} else {
		regVal = f.allocReg(DataReg, f.sizeof(instr))
	}
	asm, regX, regY, err := f.asmBinOpLoadXY(instr)
	if err != nil {
		return asm, err
	}
	switch instr.Op {
	default:
		panic(fmt.Sprintf("Unknown op (%v) in asmBinOp", instr.Op))
	case token.ADD, token.SUB, token.MUL, token.QUO, token.REM:
		asm += asmArithOp(f.Indent, instr.Op, regX, regY, &regVal)
	case token.AND, token.OR, token.XOR, token.SHL, token.SHR, token.AND_NOT:
		asm += asmBitwiseOp(f.Indent, instr.Op, regX, regY, &regVal)
	case token.EQL, token.NEQ, token.LEQ, token.GEQ, token.LSS, token.GTR:
		asm += asmCmpOp(f.Indent, instr.Op, regX, regY, &regVal)
	}
	f.freeReg(*regX)
	f.freeReg(*regY)

	addr, ok := f.ssaNames[instr.Name()]
	if !ok {
		panic(fmt.Sprintf("Unknown name (%v) in asmBinOp, instr (%v)\n", instr.Name(), instr))
	}

	a, err := f.asmStoreReg(&regVal, &addr, 0)
	if err != nil {
		return asm, err
	} else {
		asm += a
	}
	f.freeReg(regVal)

	asm = fmt.Sprintf(f.Indent+"// BEGIN ssa.BinOp, %v = %v\n", instr.Name(), instr) + asm
	asm += fmt.Sprintf(f.Indent+"// END ssa.BinOp, %v = %v\n", instr.Name(), instr)
	return asm, nil
}

func (f *Function) asmBinOpLoadXY(instr *ssa.BinOp) (asm string, x *register, y *register, err *Error) {

	if err = f.allocValueOnDemand(instr); err != nil {
		return "", nil, nil, err
	}
	if err = f.allocValueOnDemand(instr.X); err != nil {
		return "", nil, nil, err
	}
	if err = f.allocValueOnDemand(instr.Y); err != nil {
		return "", nil, nil, err
	}

	xtmp := f.allocReg(DataReg, f.sizeof(instr.X))
	x = &xtmp
	ytmp := f.allocReg(DataReg, f.sizeof(instr.Y))
	y = &ytmp
	asm = ""
	if a, err := f.asmLoadValue(instr.X, 0, f.sizeof(instr.X), x); err != nil {
		return "", nil, nil, err
	} else {
		asm += a
	}
	if a, err := f.asmLoadValue(instr.Y, 0, f.sizeof(instr.Y), y); err != nil {
		return "", nil, nil, err
	} else {
		asm += a
	}
	return asm, x, y, nil
}

func (f *Function) sizeof(val ssa.Value) uint {
	if _, ok := val.(*ssa.Const); ok {
		return f.sizeofConst(val.(*ssa.Const))
	}
	info, ok := f.ssaNames[val.Name()]
	if !ok {
		panic(fmt.Sprintf("Unknown name (%v) in asmLoadValue, value (%v)\n", val.Name(), val))
	}
	_, _, size := info.MemRegOffsetSize()
	return size
}

func (f *Function) sizeofConst(cnst *ssa.Const) uint {
	return sizeof(cnst.Type())
}

func (f *Function) asmLoadValue(val ssa.Value, offset uint, size uint, reg *register) (string, *Error) {
	if _, ok := val.(*ssa.Const); ok {
		return f.asmLoadConstValue(val.(*ssa.Const), reg)
	}
	info, ok := f.ssaNames[val.Name()]
	if !ok {
		panic(fmt.Sprintf("Unknown name (%v) in asmLoadValue, value (%v)\n", val.Name(), val))
	}
	// TODO handle non 64 bit values
	r, roffset, rsize := info.MemRegOffsetSize()
	if (rsize%8) != 0 || size != 8 {
		panic(fmt.Sprintf("Non 64bit sized (%v) value in asmLoadValue, value (%v), name (%v)\n", size, val, val.Name()))
	}
	return asmMovMemReg(f.Indent, info.name, roffset+offset, &r, reg), nil
}

func (f *Function) asmStoreReg(reg *register, addr *nameInfo, offset uint) (string, *Error) {
	// TODO handle non 64 bit values
	r, roffset, rsize := addr.MemRegOffsetSize()
	// byte sized values are not supported
	if rsize == 1 {
		rsize = 8
	}
	if (rsize % 8) != 0 {
		panic(fmt.Sprintf("Non multiple of 8 byte sized (%v) value in asmStoreReg, addr (%v), name (%v)\n", rsize, addr, addr.name))
	}
	return asmMovRegMem(f.Indent, reg, addr.name, &r, offset+roffset), nil
}

func (f *Function) asmLoadConstValue(cnst *ssa.Const, r *register) (string, *Error) {
	cnstValue := cnst.Uint64()
	return asmMovImm64Reg(f.Indent, cnstValue, r), nil
}

func (f *Function) asmUnOp(instr *ssa.UnOp) (string, *Error) {
	var err *Error
	asm := ""
	switch instr.Op {
	default:
		panic(fmt.Sprintf("Unknown Op token (%v) in asmUnOp: \"%v\"", instr.Op, instr))
	case token.NOT: // logical negation
		asm, err = f.asmUnOpNot(instr)
	case token.XOR: //bitwise complement
		asm, err = f.asmUnOpXor(instr)
	case token.SUB: // arithmetic negation
		asm, err = f.asmUnOpSub(instr)
	case token.MUL: //pointer indirection
		asm, err = f.asmUnOpPointer(instr)
	}
	asm = f.Indent + fmt.Sprintf("// BEGIN ssa.UnOp: %v = %v\n", instr.Name(), instr) + asm
	asm += f.Indent + fmt.Sprintf("// END ssa.UnOp: %v = %v\n", instr.Name(), instr)
	return asm, err

}

// logical negation
func (f *Function) asmUnOpNot(instr *ssa.UnOp) (string, *Error) {
	// TODO
	return fmt.Sprintf(f.Indent+"// instr %v\n", instr), nil
}

//bitwise complement
func (f *Function) asmUnOpXor(instr *ssa.UnOp) (string, *Error) {
	// TODO
	return fmt.Sprintf(f.Indent+"// instr %v\n", instr), nil
}

// arithmetic negation
func (f *Function) asmUnOpSub(instr *ssa.UnOp) (string, *Error) {
	// TODO
	return fmt.Sprintf(f.Indent+"// instr %v\n", instr), nil
}

//pointer indirection
func (f *Function) asmUnOpPointer(instr *ssa.UnOp) (string, *Error) {
	assignment, ok := f.ssaNames[instr.Name()]
	xName := instr.X.Name()
	xInfo, okX := f.ssaNames[xName]

	if !okX {
		panic(fmt.Sprintf("Unknown name for UnOp X (%v), instr \"(%v)\"", instr.X, instr))
	}
	if xInfo.local == nil && xInfo.param == nil && !xInfo.IsPointer() {
		panic(fmt.Sprintf("In UnOp, X (%v) isn't a pointer, X.type (%v), instr \"(%v)\"", instr.X, instr.X.Type(), instr))
	}
	asm := ""
	if !ok {
		info, err := f.asmAllocLocal(instr.Name(), instr.Type())
		if err != nil {
			panic(fmt.Sprintf("Err in UnOp X (%v), instr \"(%v)\", msg: \"%v\"", instr.X, instr, err))
		}
		assignment = info
		/*if xInfo.local == nil && xInfo.param == nil {
			assignment.typ = xInfo.PointerUnderlyingType()
		} else {
			assignment.typ = xInfo.typ
		}*/
	}
	xReg, xOffset, xSize := xInfo.MemRegOffsetSize()
	aReg, aOffset, aSize := assignment.MemRegOffsetSize()
	if xSize != aSize {
		panic("xSize := aSize in asmUnOpPointer")
	}
	size := aSize
	tmp1 := f.allocReg(DataReg, DataRegSize)
	tmp2 := f.allocReg(DataReg, DataRegSize)
	asm += asmMovMemIndirectMem(f.Indent, xInfo.name, xOffset, &xReg, assignment.name, aOffset, &aReg, size, &tmp1, &tmp2)
	f.ssaNames[assignment.name] = assignment
	f.freeReg(tmp1)
	f.freeReg(tmp2)
	return asm, nil
}

func (f *Function) asmIndexAddr(instr *ssa.IndexAddr) (string, *Error) {
	if instr == nil {
		return "", &Error{Err: errors.New("asmIndexAddr: nil instr"), Pos: instr.Pos()}

	}
	asm := ""
	constIndex := false
	paramIndex := false
	var cnst *ssa.Const
	var param *ssa.Parameter
	switch instr.Index.(type) {
	default:
	case *ssa.Const:
		constIndex = true
		cnst = instr.Index.(*ssa.Const)
	case *ssa.Parameter:
		paramIndex = true
		param = instr.Index.(*ssa.Parameter)
	}

	xInfo := f.ssaNames[instr.X.Name()]

	// TODO check if xInfo is pointer, array, struct, etc.
	//if xInfo.IsPointer() || xInfo.IsArray() {

	/*if xInfo.reg == nil {
		msg := fmt.Sprintf("nil xInfo.reg (%v) in indexaddr op", xInfo.name)
		return asm, &Error{Err: errors.New(msg), Pos: instr.Pos()}
	}*/

	assignment, ok := f.ssaNames[instr.Name()]
	if !ok {
		local, err := f.asmAllocLocal(instr.Name(), instr.Type())
		if err != nil {
			msg := fmt.Errorf("err in indexaddr op, msg:\"%v\"", err)
			return asm, &Error{Err: msg, Pos: instr.Pos()}
		}
		assignment = local
		f.ssaNames[instr.Name()] = assignment
	}

	if constIndex {
		tmpReg := f.allocReg(DataReg, pointerSize)
		size := uint(sizeofElem(xInfo.typ))
		idx := uint(cnst.Uint64())
		xReg, xOffset, _ := xInfo.MemRegOffsetSize()
		assignmentReg, assignmentOffset, _ := assignment.MemRegOffsetSize()
		asm += asmLea(f.Indent, xInfo.name, xOffset+idx*size, &xReg, &tmpReg)
		asm += asmMovRegMem(f.Indent, &tmpReg, assignment.name, &assignmentReg, assignmentOffset)
		f.freeReg(tmpReg)
	} else if paramIndex {
		p := f.ssaNames[param.Name()]
		tmpReg := f.allocReg(DataReg, pointerSize)
		tmp2Reg := f.allocReg(DataReg, pointerSize)
		xReg, xOffset, _ := xInfo.MemRegOffsetSize()
		pReg, pOffset, pSize := p.MemRegOffsetSize()
		if pSize != 8 {
			fmt.Println("instr:", instr)
			fmt.Println("pSize:", pSize)
			panic("Index size not 8 bytes in asmIndexAddr")
		}
		assignmentReg, assignmentOffset, _ := assignment.MemRegOffsetSize()
		asm += asmMovMemReg(f.Indent, p.name, pOffset, &pReg, &tmp2Reg)
		asm += asmLea(f.Indent, xInfo.name, xOffset, &xReg, &tmpReg)
		asm += asmAddRegReg(f.Indent, &tmpReg, &tmp2Reg)
		asm += asmMovRegMem(f.Indent, &tmp2Reg, assignment.name, &assignmentReg, assignmentOffset)
		f.freeReg(tmpReg)
		f.freeReg(tmp2Reg)

	} else {
		asm = fmt.Sprintf(f.Indent+"// Unsupported ssa.IndexAddr:%v\n", instr)
	}
	f.ssaNames[instr.Name()] = assignment
	asm = f.Indent + fmt.Sprintf("// BEGIN ssa.IndexAddr: %v = %v\n", instr.Name(), instr) + asm
	asm += f.Indent + fmt.Sprintf("// END ssa.IndexAddr: %v = %v\n", instr.Name(), instr)
	return asm, nil
}

func (f *Function) asmAllocInstr(instr *ssa.Alloc) (string, *Error) {
	asm := ""
	if instr == nil {
		return "", &Error{Err: errors.New("asmAllocInstr: nil instr"), Pos: instr.Pos()}

	}
	if instr.Heap {
		return "", &Error{Err: errors.New("asmAllocInstr: heap alloc"), Pos: instr.Pos()}
	}

	//Alloc values are always addresses, and have pointer types, so the type
	//of the allocated variable is actually
	//Type().Underlying().(*types.Pointer).Elem().
	info := f.ssaNames[instr.Name()]
	if info.local == nil {
		panic(fmt.Sprintf("Expect %v to be a local variable", instr.Name()))
	}
	if _, ok := info.typ.(*types.Pointer); ok {
	} else {
	}
	f.ssaNames[instr.Name()] = info
	return asm, nil
}

func (f *Function) asmValue(value ssa.Value, dstReg *register, dstVar *varInfo) string {
	if dstReg == nil && dstVar == nil {
		panic("Both dstReg & dstVar are nil!")
	}
	if dstReg != nil && dstVar != nil {
		panic("Both dstReg & dstVar are non nil!")
	}
	if dstReg != nil {
		// TODO
	}
	if dstVar != nil {
		// TODO
	}
	return ""
}

func (f *Function) localsSize() uint32 {
	size := uint32(0)
	for _, name := range f.ssaNames {
		if name.local != nil {
			size += uint32(name.local.size)
		}
	}
	return size
}

func (f *Function) init() *Error {
	f.registers = make(map[string]bool)
	f.ssaNames = make(map[string]nameInfo)
	f.phiInfo = make(map[int]map[int][]phiInfo)
	f.initRegs()
	return nil
}

func (f *Function) initRegs() {
	for _, r := range registers {
		f.registers[r.name] = false
	}
}

// size in bytes
func (f *Function) allocReg(t RegType, size uint) register {
	var reg register
	found := false
	for i := 0; i < len(registers); i++ {
		r := registers[i]
		if f.excludeReg(&r) {
			continue
		}
		used := f.registers[r.name]
		// r.width is in bits so multiple size (which is in bytes) by 8
		if !used && r.typ == t && r.width == size*8 {
			reg = r
			found = true
			break
		}
	}
	if found {
		f.registers[reg.name] = true
	} else {
		// any of the data registers can be used as an address register on x86_64
		if t == AddrReg {
			return f.allocReg(DataReg, size)
		} else {
			panic(fmt.Sprintf("couldn't alloc register, type: %v, width in bits: %v, size in bytes:%v", t, size*8, size))
		}
	}
	return reg
}

func (f *Function) excludeReg(reg *register) bool {
	for _, r := range excludedRegisters {
		if r.name == reg.name {
			return true
		}
	}
	return false
}

// zeroReg returns the assembly for zeroing the passed in register
func (f *Function) zeroReg(r *register) string {
	return asmZeroReg(f.Indent, r)
}

func (f *Function) freeReg(reg register) {
	f.registers[reg.name] = false
}

// paramsSize returns the size of the parameters in bytes
func (f *Function) paramsSize() uint {
	size := uint(0)
	for _, p := range f.ssa.Params {
		size += sizeof(p.Type())
	}
	return size
}

func retName() string {
	return "ret0"
}

// retType gives the return type
func (f *Function) retType() types.Type {
	results := f.ssa.Signature.Results()
	if results.Len() == 0 {
		return nil
	}
	if results.Len() > 1 {
		panic("Functions with more than one return value not supported")
	}
	return results.At(0).Type()
}

func (f *Function) retParam() *paramInfo {
	return &paramInfo{name: retName(), offset: f.retOffset(), size: f.retSize(), info: nil, extra: nil}
}

// retSize returns the size of the return value in bytes
func (f *Function) retSize() uint {
	size := sizeof(f.retType())
	return size
}

// retOffset returns the offset of the return value in bytes
func (f *Function) retOffset() uint {
	return f.paramsSize()
}

func (f *Function) allocValueOnDemand(v ssa.Value) *Error {
	_, ok := f.ssaNames[v.Name()]
	if ok {
		return nil
	}
	switch v.(type) {
	case *ssa.Const:
		return nil
	}
	if !ok {
		local, err := f.asmAllocLocal(v.Name(), v.Type())
		if err != nil {
			msg := fmt.Errorf("err in allocValueOnDemand, msg:\"%v\"", err)
			return &Error{Err: msg, Pos: v.Pos()}
		}
		f.ssaNames[v.Name()] = local
	}
	return nil
}

var pointerSize = uint(8)
var sliceSize = uint(24)

type simdInfo struct {
	name     string
	size     uint
	elemSize uint
}

func simdReflect(t reflect.Type) simdInfo {
	elemSize := uint(0)
	if t.Kind() == reflect.Array {
		elemSize = uint(t.Elem().Size())
	}
	return simdInfo{t.Name(), uint(t.Size()), elemSize}
}

func simdTypes() []simdInfo {
	simdInt := reflect.TypeOf(simd.Int(0))
	simdInt4 := reflect.TypeOf(simd.Int4{})
	return []simdInfo{simdReflect(simdInt), simdReflect(simdInt4)}
}

func isSimd(t types.Type) bool {
	if t, ok := t.(*types.Named); ok {
		tname := t.Obj()
		for _, simdType := range simdTypes() {
			if tname.Name() == simdType.name {
				return true
			}
		}
	}
	return false
}

func simdTypeInfo(t types.Type) (simdInfo, error) {
	if !isSimd(t) {
		msg := fmt.Errorf("type (%v) is not simd type", t.String())
		return simdInfo{}, msg
	}
	named := t.(*types.Named)
	tname := named.Obj()
	for _, simdType := range simdTypes() {
		if tname.Name() == simdType.name {
			return simdType, nil
		}
	}
	msg := fmt.Errorf("type (%v) couldn't find simd type info", t.String())
	return simdInfo{}, msg
}

func simdHasElemSize(t types.Type) bool {
	if simdInfo, err := simdTypeInfo(t); err == nil {
		return simdInfo.elemSize > 0
	} else {
		msg := fmt.Sprintf("Error in simdHasElemSize, type (%v) is not simd", t.String())
		panic(msg)
	}
}

func simdElemSize(t types.Type) uint {
	if simdInfo, err := simdTypeInfo(t); err == nil {
		return simdInfo.elemSize
	} else {
		msg := fmt.Sprintf("Error in simdElemSize, type (%v) is not simd", t.String())
		panic(msg)
	}
}

func sizeofElem(t types.Type) uint {
	var e types.Type
	switch t := t.(type) {
	default:
		panic(fmt.Sprintf("t (%v) not an array or slice type\n", t.String()))
	case *types.Slice:
		e = t.Elem()
	case *types.Array:
		e = t.Elem()
	case *types.Named:
		if isSimd(t) && simdHasElemSize(t) {
			return simdElemSize(t)
		}
		panic(fmt.Sprintf("t (%v), isSimd (%v)\n", t.String(), isSimd(t)))
	}
	return sizeof(e)
}

func sizeof(t types.Type) uint {

	switch t := t.(type) {
	default:
		fmt.Println("t:", t)
		panic("Error unknown type in sizeof")
	case *types.Tuple:
		// TODO: fix, usage of reflect is wrong!
		return uint(reflect.TypeOf(t).Elem().Size())
	case *types.Basic:
		return sizeBasic(t)
	case *types.Pointer:
		return pointerSize
	case *types.Slice:
		return sliceSize
	case *types.Array:
		// TODO: fix, calculation most likely wrong
		return uint(t.Len()) * sizeof(t.Elem())
	case *types.Named:
		if !isSimd(t) {

		}
		if info, err := simdTypeInfo(t); err != nil {
			panic(fmt.Sprintf("Error unknown type in sizeof err:\"%v\"", err))
		} else {
			return info.size
		}
	}
}

func intSize() uint {
	return uint(reflect.TypeOf(int(1)).Size())
}

func uintSize() uint {
	return uint(reflect.TypeOf(uint(1)).Size())
}

func boolSize() uint {
	return uint(reflect.TypeOf(true).Size())
}

func ptrSize() uint {
	return pointerSize
}

// sizeBasic return the size in bytes of a basic type
func sizeBasic(b *types.Basic) uint {
	switch b.Kind() {
	default:
		panic("Unknown basic type")
	case types.Bool:
		return uint(reflect.TypeOf(true).Size())
	case types.Int:
		return uint(reflect.TypeOf(int(1)).Size())
	case types.Int8:
		return uint(reflect.TypeOf(int8(1)).Size())
	case types.Int16:
		return uint(reflect.TypeOf(int16(1)).Size())
	case types.Int32:
		return uint(reflect.TypeOf(int32(1)).Size())
	case types.Int64:
		return uint(reflect.TypeOf(int64(1)).Size())
	case types.Uint:
		return uint(reflect.TypeOf(uint(1)).Size())
	case types.Uint8:
		return uint(reflect.TypeOf(uint8(1)).Size())
	case types.Uint16:
		return uint(reflect.TypeOf(uint16(1)).Size())
	case types.Uint32:
		return uint(reflect.TypeOf(uint32(1)).Size())
	case types.Uint64:
		return uint(reflect.TypeOf(uint64(1)).Size())
	case types.Float32:
		return uint(reflect.TypeOf(float32(1)).Size())
	case types.Float64:
		return uint(reflect.TypeOf(float64(1)).Size())
	}
}

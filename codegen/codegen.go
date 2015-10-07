package codegen

import (
	"errors"
	"fmt"
	"go/token"
	"log"
	"strconv"
	"strings"

	"golang.org/x/tools/go/types"

	"reflect"
	"unsafe"

	"github.com/bjwbell/gensimd/codegen/instructionsetxml"
	"github.com/bjwbell/gensimd/simd"

	"golang.org/x/tools/go/ssa"
)

type Function struct {
	Indent         string
	ssa            *ssa.Function
	instructionset *instructionsetxml.Instructionset
	registers      map[register]bool // maps register to false if unused and true if used
	ssaNames       map[string]nameInfo
}

type nameInfo struct {
	name  string
	typ   types.Type
	reg   *register
	local *varInfo
	param *paramInfo
}

func (name *nameInfo) IsSsaLocal() bool {
	return name.local != nil && name.local.info != nil
}

func (name *nameInfo) IsPointer() bool {
	_, ok := name.typ.(*types.Pointer)
	return ok
}

func (name *nameInfo) IsArray() bool {
	_, ok := name.typ.(*types.Array)
	return ok
}

type varInfo struct {
	name string
	// offset from the stack pointer (SP)
	offset int
	size   int
	info   *ssa.Alloc
	reg    *register
}

func (v *varInfo) Reg() (*register, error) {
	if v.reg != nil {
		return v.reg, nil
	} else {
		return nil, errors.New("varInfo has no reg set")
	}
}

func (v *varInfo) ssaName() string {
	return v.info.Name()
}

type paramInfo struct {
	name string
	// offset from the frame pointer (FP)
	offset int
	size   int
	info   *ssa.Parameter
	extra  interface{}
}

func (p *paramInfo) Reg() (*register, error) {
	if p.extra != nil {
		return &p.extra.(*paramSlice).reg, nil
	} else {
		return nil, errors.New("param p has no register set")
	}
}

func (p *paramInfo) ssaName() string {
	return p.info.Name()
}

type paramSlice struct {
	offset      int
	lenOffset   int
	reg         register
	regValid    bool
	lenReg      register
	lenRegValid bool
}

type Error struct {
	Err error
	Pos token.Pos
}

func CreateFunction(instructionsetPath string, fn *ssa.Function) (*Function, *Error) {
	if fn == nil {
		return nil, &Error{Err: errors.New("Nil function passed in")}
	}
	instructionset, err := instructionsetxml.LoadInstructionset(instructionsetPath)
	if err != nil {
		return nil, &Error{Err: err}
	}
	f := Function{ssa: fn, instructionset: instructionset}
	f.Indent = "        "
	f.init()
	return &f, nil
}

func (f *Function) GoAssembly() (string, *Error) {
	return f.asmFunc()
}

func memFn(name string, offset int, regName string) func() string {
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
	// offset in bytes
	offset := 0
	asm := ""
	for _, p := range f.ssa.Params {
		param := paramInfo{name: p.Name(), offset: offset, info: p, size: sizeof(p.Type())}
		// TODO alloc reg based on other param types
		if _, ok := p.Type().(*types.Slice); ok {
			opMem := Operand{Type: OperandType(M64), Input: true, Output: false, Value: memFn(param.name, offset, "FP")}
			reg := f.allocReg(AddrReg, pointerSize)
			opReg := Operand{Type: OperandType(R64), Input: false, Output: true, Value: regFn(reg.name)}
			ops := []*Operand{&opMem, &opReg}
			if a, err := InstrAsm(f.instructionset, GetInstrType(TMOV), ops); err != nil {
				return "", &Error{err, p.Pos()}
			} else {
				asm += f.Indent + a + "\n"
			}
			// TODO is sizeof length data always pointer size?
			lenReg := f.allocReg(AddrReg, pointerSize)
			opMem.Value = memFn(param.name+"len", offset+pointerSize, "FP")
			opReg.Value = regFn(lenReg.name)
			if a, err := InstrAsm(f.instructionset, GetInstrType(TMOV), ops); err != nil {
				return "", &Error{err, p.Pos()}
			} else {
				asm += f.Indent + a + "\n"
			}
			param.extra = paramSlice{offset: offset, reg: reg, regValid: true, lenReg: lenReg, lenRegValid: true}
		} else if basic, ok := p.Type().(*types.Basic); ok && basic.Kind() == types.Int {
			opMem := Operand{Type: OperandType(M64), Input: true, Output: false, Value: memFn(param.name, offset, "FP")}
			reg := f.allocReg(DataReg, intSize())
			opReg := Operand{Type: OperandType(R64), Input: false, Output: true, Value: regFn(reg.name)}
			ops := []*Operand{&opMem, &opReg}
			if a, err := InstrAsm(f.instructionset, GetInstrType(TMOV), ops); err != nil {
				return "", &Error{err, p.Pos()}
			} else {
				asm += f.Indent + a + "\n"
			}
		} else {
			return "", &Error{Err: errors.New("Unsupported param type"), Pos: p.Pos()}
		}
		f.ssaNames[param.name] = nameInfo{name: param.name, typ: param.info.Type(), local: nil, param: &param}
		offset += param.size
	}
	return asm, nil
}

func (f *Function) asmFunc() (string, *Error) {
	asm, err := f.asmParams()
	if err != nil {
		return "", err
	}
	zeroRetValue, err := f.asmZeroRetValue()
	if err != nil {
		return "", err
	}
	zeroSsaLocals, err := f.asmZeroSsaLocals()
	if err != nil {
		return "", err
	}
	basicblocks, err := f.asmBasicBlocks()
	if err != nil {
		return "", err
	}
	zeroNonSsaLocals, err := f.asmZeroNonSsaLocals()
	if err != nil {
		return "", err
	}
	frameSize := f.localsSize()
	asm += zeroRetValue
	asm += zeroSsaLocals
	asm += zeroNonSsaLocals
	asm += basicblocks

	a := fmt.Sprintf("TEXT ·%v(SB),NOSPLIT,$%v-%v\n%v", f.ssa.Name(), frameSize, f.paramsSize()+f.retSize(), asm)
	return a, nil
}

func (f *Function) asmZeroSsaLocals() (string, *Error) {
	asm := ""
	offset := 0
	locals := f.ssa.Locals
	for _, local := range locals {
		if local.Heap {
			msg := errors.New(fmt.Sprintf("Can't heap alloc local, name: %v", local.Name()))
			return "", &Error{Err: msg, Pos: local.Pos()}
		}
		sp := register{"SP", AddrReg, pointerSize * 8}

		//local values are always addresses, and have pointer types, so the type
		//of the allocated variable is actually
		//Type().Underlying().(*types.Pointer).Elem().
		typ := local.Type().Underlying().(*types.Pointer).Elem()
		size := sizeof(typ)
		asm += asmZeroMemory(f.Indent, local.Name(), offset, size, sp)
		v := varInfo{name: local.Name(), offset: offset, size: size, info: local}
		f.ssaNames[v.name] = nameInfo{name: v.name, typ: typ, reg: nil, local: &v, param: nil}
		offset += size
	}
	return asm, nil
}

func (f *Function) asmZeroNonSsaLocals() (string, *Error) {
	asm := ""
	for _, name := range f.ssaNames {
		if name.local == nil || name.IsSsaLocal() {
			continue
		}
		sp := register{"SP", AddrReg, pointerSize * 8}
		asm += asmZeroMemory(f.Indent, name.name, name.local.offset, name.local.size, sp)
	}
	return asm, nil
}

func (f *Function) asmZeroRetValue() (string, *Error) {
	asm := asmZeroMemory(f.Indent, "~ret", f.retOffset(), f.retSize(), register{"FP", AddrReg, pointerSize * 8})
	return asm, nil
}

func (f *Function) asmBasicBlocks() (string, *Error) {
	asm := ""
	for i := 0; i < len(f.ssa.Blocks); i++ {
		asm += f.asmBasicBlock(f.ssa.Blocks[i])
	}
	return asm, nil
}

func (f *Function) asmBasicBlock(block *ssa.BasicBlock) string {
	asm := strconv.Itoa(block.Index) + ":\n"
	for i := 0; i < len(block.Instrs); i++ {

		asm += f.asmInstr(block.Instrs[i])
	}
	return asm
}

func (f *Function) asmInstr(instr ssa.Instruction) string {
	if instr == nil {
		panic("Nil instr in asmInstr")
	}
	asm := ""
	switch instr.(type) {
	default:
		asm += f.Indent + fmt.Sprintf("Unknown ssa instruction: %v", instr)
	case *ssa.Alloc:
		i := instr.(*ssa.Alloc)
		if a, err := f.asmAllocInstr(i); err != nil {
			log.Fatal("Error in f.asmAllocInstr")
			return ""
		} else {
			asm += a
		}
	case *ssa.BinOp:
		i := instr.(*ssa.BinOp)
		asm += f.Indent + fmt.Sprintf("ssa.BinOp: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.Call:
		i := instr.(*ssa.Call)
		asm += f.Indent + fmt.Sprintf("ssa.Call: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.ChangeInterface:
		i := instr.(*ssa.ChangeInterface)
		asm += f.Indent + fmt.Sprintf("ssa.ChangeInterface: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.ChangeType:
		i := instr.(*ssa.ChangeType)
		asm += f.Indent + fmt.Sprintf("ssa.ChangeType: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.Convert:
		i := instr.(*ssa.Convert)
		asm += f.Indent + fmt.Sprintf("ssa.Convert: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.Defer:
		asm += f.Indent + fmt.Sprintf("ssa.Defer: %v", instr) + "\n"
	case *ssa.Extract:
		i := instr.(*ssa.Extract)
		asm += f.Indent + fmt.Sprintf("ssa.Extra: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.Field:
		i := instr.(*ssa.Field)
		asm += f.Indent + fmt.Sprintf("ssa.Field: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.FieldAddr:
		i := instr.(*ssa.FieldAddr)
		asm += f.Indent + fmt.Sprintf("ssa.FieldAddr: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.Go:
		asm += f.Indent + fmt.Sprintf("ssa.Go: %v", instr) + "\n"
	case *ssa.If:
		asm += f.Indent + fmt.Sprintf("ssa.If: %v", instr) + "\n"
	case *ssa.Index:
		i := instr.(*ssa.Index)
		asm += f.Indent + fmt.Sprintf("ssa.Index: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.IndexAddr:
		i := instr.(*ssa.IndexAddr)
		if a, err := f.asmIndexAddrInstr(i); err != nil {
			log.Fatal("Error in f.asmIndexAddrInstr")
			return ""
		} else {
			asm += f.Indent + fmt.Sprintf("// ssa.IndexAddr: %v, name: %v", i, i.Name()) + "\n"
			asm += a
		}
	case *ssa.Jump:
		jmp := instr.(*ssa.Jump)
		asm += f.Indent + strings.Replace(jmp.String(), "jump", "JMP ", -1) + "\n"
	case *ssa.Lookup:
		i := instr.(*ssa.Lookup)
		asm += f.Indent + fmt.Sprintf("ssa.Lookup: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.MakeChan:
		i := instr.(*ssa.MakeChan)
		asm += f.Indent + fmt.Sprintf("ssa.MakeChan: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.MakeClosure:
		i := instr.(*ssa.MakeClosure)
		asm += f.Indent + fmt.Sprintf("ssa.MakeClosure: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.MakeInterface:
		i := instr.(*ssa.MakeInterface)
		asm += f.Indent + fmt.Sprintf("ssa.MakeInterface: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.MakeMap:
		i := instr.(*ssa.MakeMap)
		asm += f.Indent + fmt.Sprintf("ssa.MakeMap: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.MakeSlice:
		i := instr.(*ssa.MakeSlice)
		asm += f.Indent + fmt.Sprintf("ssa.MakeSlice: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.MapUpdate:
		asm += f.Indent + fmt.Sprintf("ssa.MapUpdate: %v", instr) + "\n"
	case *ssa.Next:
		i := instr.(*ssa.Next)
		asm += f.Indent + fmt.Sprintf("ssa.Next: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.Panic:
		asm += f.Indent + fmt.Sprintf("ssa.Panic: %v", instr) + "\n"
	case *ssa.Phi:
		i := instr.(*ssa.Phi)
		asm += f.Indent + fmt.Sprintf("ssa.Phi: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.Range:
		i := instr.(*ssa.Range)
		asm += f.Indent + fmt.Sprintf("ssa.Range: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.Return:
		asm += f.Indent + fmt.Sprintf("ssa.Return: %v", instr) + "\n"
	case *ssa.RunDefers:
		asm += f.Indent + fmt.Sprintf("ssa.RunDefers: %v", instr) + "\n"
	case *ssa.Select:
		i := instr.(*ssa.Select)
		asm += f.Indent + fmt.Sprintf("ssa.Select: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.Send:
		asm += f.Indent + fmt.Sprintf("ssa.Send: %v", instr) + "\n"
	case *ssa.Slice:
		i := instr.(*ssa.Slice)
		asm += f.Indent + fmt.Sprintf("ssa.Slice: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.Store:
		i := instr.(*ssa.Store)
		asm += f.Indent + fmt.Sprintf("ssa.Store: %v, addr: %v, val: %v", instr, i.Addr, i.Val) + "\n"
	case *ssa.TypeAssert:
		i := instr.(*ssa.TypeAssert)
		asm += f.Indent + fmt.Sprintf("ssa.TypeAssert: %v, name: %v", instr, i.Name()) + "\n"
	case *ssa.UnOp:
		i := instr.(*ssa.UnOp)
		asm += f.Indent + fmt.Sprintf("ssa.UnOp: %v, name: %v", instr, i.Name()) + "\n"
	}
	return asm
}

func (f *Function) asmIndexAddrInstr(instr *ssa.IndexAddr) (string, *Error) {
	if instr == nil {
		return "", &Error{Err: errors.New("asmIndexAddrInstr: nil instr"), Pos: instr.Pos()}

	}
	asm := ""
	constIndex := false
	/*regIndex := false
	localIndex := false
	paramIndex := false*/
	var cnst *ssa.Const
	switch instr.Index.(type) {
	default:
	case *ssa.Const:
		constIndex = true
		cnst = instr.Index.(*ssa.Const)
	case *ssa.Parameter:
		/*paramIndex = true
		param := instr.Index.(*ssa.Parameter)*/
	}
	assignment := f.ssaNames[instr.Name()]
	nameInfo := f.ssaNames[instr.X.Name()]
	// TODO check if nameInfo is pointer, array, struct, etc.
	//if nameInfo.IsPointer() || nameInfo.IsArray() {
	if nameInfo.reg != nil {
		if constIndex {
			if assignment.reg == nil {
				reg := f.allocReg(AddrReg, pointerSize)
				assignment.reg = &reg
			}
			tmpReg := f.allocReg(DataReg, pointerSize)
			size := uint32(sizeof(nameInfo.typ))
			idx := uint32(cnst.Uint64())
			asm = asmMovRegReg(f.Indent, nameInfo.reg, &tmpReg)
			asm += asmAddImm32Reg(f.Indent, idx*size, &tmpReg)
			asm += asmMovRegReg(f.Indent, &tmpReg, assignment.reg)
			f.freeReg(tmpReg)
		}
	}
	f.ssaNames[instr.Name()] = assignment
	return asm, nil
}

func (f *Function) asmAllocInstr(instr *ssa.Alloc) (string, *Error) {
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
		opMem := Operand{Type: OperandType(M64), Input: true, Output: false, Value: memFn(info.name, info.local.offset, "SP")}
		reg := f.allocReg(AddrReg, pointerSize)
		opReg := Operand{Type: OperandType(R64), Input: false, Output: true, Value: regFn(reg.name)}
		ops := []*Operand{&opMem, &opReg}
		if a, err := InstrAsm(f.instructionset, GetInstrType(TMOV), ops); err != nil {
			return "", &Error{err, instr.Pos()}
		} else {
			return f.Indent + a + "\n", nil
		}
	} else {
		opMem := Operand{Type: OperandType(M), Input: true, Output: false, Value: memFn(info.name, info.local.offset, "SP")}
		reg := f.allocReg(AddrReg, pointerSize)
		opReg := Operand{Type: OperandType(R64), Input: false, Output: true, Value: regFn(reg.name)}
		ops := []*Operand{&opMem, &opReg}
		if a, err := InstrAsm(f.instructionset, GetInstrType(TLEA), ops); err != nil {
			return "", &Error{err, instr.Pos()}
		} else {
			info.reg = &reg
			f.ssaNames[instr.Name()] = info
			return f.Indent + a + "\n", nil
		}

	}
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

func (f *Function) localsSize() int {
	size := 0
	for _, name := range f.ssaNames {
		if name.local != nil {
			size += sizeof(name.typ)
		}
	}
	return size
}

func (f *Function) init() *Error {
	f.registers = make(map[register]bool)
	f.ssaNames = make(map[string]nameInfo)
	f.initRegs()
	return nil
}

func (f *Function) initRegs() {
	for _, r := range registers {
		f.registers[r] = false
	}
}

// size in bytes
func (f *Function) allocReg(t RegType, size int) register {
	var reg register
	found := false
	for i := 0; i < len(registers); i++ {
		r := registers[i]
		used := f.registers[r]
		// r.width is in bits so multiple size (which is in bytes) by 8
		if !used && r.typ == t && r.width == size*8 {
			reg = r
			found = true
			break
		}
	}
	if found {
		f.registers[reg] = true
	} else {
		// any of the data registers can be used as an address register on x86_64
		if t == AddrReg {
			return f.allocReg(DataReg, size)
		} else {
			panic(fmt.Sprintf("couldn't alloc register, type: %v, size: %v", t, size*8))
		}
	}
	return reg
}

func (f *Function) freeReg(reg register) {
	f.registers[reg] = false
}

// paramsSize returns the size of the parameters in bytes
func (f *Function) paramsSize() int {
	size := 0
	for _, p := range f.ssa.Params {
		size += sizeof(p.Type())
	}
	return size
}

// retSize returns the size of the return value in bytes
func (f *Function) retSize() int {
	results := f.ssa.Signature.Results()
	if results.Len() == 0 {
		return 0
	}
	if results.Len() > 1 {
		panic("Functions with more than one return value not supported")
	}
	size := sizeof(results)
	fmt.Println("retSize:", size)
	return size
}

// retOffset returns the offset of the return value in bytes
func (f *Function) retOffset() int {
	return f.paramsSize()
}

var pointerSize = 8
var sliceSize = 24

func sizeof(t types.Type) int {

	switch t.(type) {
	default:
		fmt.Println("t:", t)
		panic("Error unknown type in sizeof")
	case *types.Tuple:
		tuple := t.(*types.Tuple)
		// TODO: fix, usage of reflect is wrong!
		return int(reflect.TypeOf(tuple).Elem().Size())
	case *types.Basic:
		basic, _ := t.(*types.Basic)
		return sizeBasic(basic)
	case *types.Pointer:
		return pointerSize
	case *types.Slice:
		return sliceSize
	case *types.Array:
		arr, _ := t.(*types.Array)
		// TODO: fix, calculation most likely wrong
		return int(arr.Len()) * sizeof(arr.Elem())
	case *types.Named:
		named, _ := t.(*types.Named)
		tname := named.Obj()
		i := simd.Int(0)
		simdInt := reflect.TypeOf(i)
		var i4 simd.Int4
		simdInt4 := reflect.TypeOf(i4)
		switch tname.Name() {
		default:
			panic("Error unknown type in sizeof")
		case simdInt.Name():
			return int(unsafe.Sizeof(i))
		case simdInt4.Name():
			return int(unsafe.Sizeof(i4))
		}
	}
}

func intSize() int {
	return int(reflect.TypeOf(int(1)).Size())
}

func uintSize() int {
	return int(reflect.TypeOf(uint(1)).Size())
}

func boolSize() int {
	return int(reflect.TypeOf(true).Size())
}

// sizeBasic return the size in bytes of a basic type
func sizeBasic(b *types.Basic) int {
	switch b.Kind() {
	default:
		panic("Unknown basic type")
	case types.Bool:
		return int(reflect.TypeOf(true).Size())
	case types.Int:
		return int(reflect.TypeOf(int(1)).Size())
	case types.Int8:
		return int(reflect.TypeOf(int8(1)).Size())
	case types.Int16:
		return int(reflect.TypeOf(int16(1)).Size())
	case types.Int32:
		return int(reflect.TypeOf(int32(1)).Size())
	case types.Int64:
		return int(reflect.TypeOf(int64(1)).Size())
	case types.Uint:
		return int(reflect.TypeOf(uint(1)).Size())
	case types.Uint8:
		return int(reflect.TypeOf(uint8(1)).Size())
	case types.Uint16:
		return int(reflect.TypeOf(uint16(1)).Size())
	case types.Uint32:
		return int(reflect.TypeOf(uint32(1)).Size())
	case types.Uint64:
		return int(reflect.TypeOf(uint64(1)).Size())
	case types.Float32:
		return int(reflect.TypeOf(float32(1)).Size())
	case types.Float64:
		return int(reflect.TypeOf(float64(1)).Size())
	}
}

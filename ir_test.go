//===- ir_test.go - Tests for ir ------------------------------------------===//
//
// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
//===----------------------------------------------------------------------===//
//
// This file tests bindings for the ir component.
//
//===----------------------------------------------------------------------===//

package llvm

import (
	"strconv"
	"strings"
	"testing"
)

func testAttribute(t *testing.T, name string) {
	ctx := NewContext()
	mod := ctx.NewModule("")
	defer mod.Dispose()

	ftyp := FunctionType(ctx.VoidType(), nil, false)
	fn := AddFunction(mod, "foo", ftyp)

	kind := AttributeKindID(name)
	attr := mod.Context().CreateEnumAttribute(kind, 0)

	fn.AddFunctionAttr(attr)
	newattr := fn.GetEnumFunctionAttribute(kind)
	if attr != newattr {
		t.Errorf("got attribute %p, want %p", newattr.C, attr.C)
	}

	text := mod.String()
	if !strings.Contains(text, " "+name+" ") {
		t.Errorf("expected attribute '%s', got:\n%s", name, text)
	}

	fn.RemoveEnumFunctionAttribute(kind)
	newattr = fn.GetEnumFunctionAttribute(kind)
	if !newattr.IsNil() {
		t.Errorf("got attribute %p, want 0", newattr.C)
	}
}

func TestAttributes(t *testing.T) {
	// Tests that our attribute constants haven't drifted from LLVM's.
	attrTests := []string{
		"sanitize_address",
		"alwaysinline",
		"builtin",
		"convergent",
		"inlinehint",
		"inreg",
		"jumptable",
		"minsize",
		"naked",
		"nest",
		"noalias",
		"nobuiltin",
		"noduplicate",
		"noimplicitfloat",
		"noinline",
		"nonlazybind",
		"nonnull",
		"noredzone",
		"noreturn",
		"nounwind",
		"optnone",
		"optsize",
		"readnone",
		"readonly",
		"returned",
		"returns_twice",
		"signext",
		"safestack",
		"ssp",
		"sspreq",
		"sspstrong",
		"sanitize_thread",
		"sanitize_memory",
		"uwtable",
		"zeroext",
		"cold",
		"nocf_check",
	}

	for _, name := range attrTests {
		majorVersion, err := strconv.Atoi(strings.SplitN(Version, ".", 2)[0])
		if err != nil {
			// sanity check, should be unreachable
			t.Errorf("could not parse LLVM version: %v", err)
		}
		if majorVersion >= 15 && name == "uwtable" {
			// This changed from an EnumAttr to an IntAttr in LLVM 15, and testAttribute doesn't work on such attributes.
			continue
		}
		testAttribute(t, name)
	}
}

func TestDebugLoc(t *testing.T) {
	ctx := NewContext()
	mod := ctx.NewModule("")
	defer mod.Dispose()

	b := ctx.NewBuilder()
	defer b.Dispose()

	d := NewDIBuilder(mod)
	defer func() {
		d.Destroy()
	}()
	file := d.CreateFile("dummy_file", "dummy_dir")
	voidInfo := d.CreateBasicType(DIBasicType{Name: "void"})
	typeInfo := d.CreateSubroutineType(DISubroutineType{
		File:       file,
		Parameters: []Metadata{voidInfo},
		Flags:      0,
	})
	scope := d.CreateFunction(file, DIFunction{
		Name:         "foo",
		LinkageName:  "foo",
		Line:         10,
		ScopeLine:    10,
		Type:         typeInfo,
		File:         file,
		IsDefinition: true,
	})

	b.SetCurrentDebugLocation(10, 20, scope, Metadata{})
	loc := b.GetCurrentDebugLocation()
	if loc.Line != 10 {
		t.Errorf("Got line %d, though wanted 10", loc.Line)
	}
	if loc.Col != 20 {
		t.Errorf("Got column %d, though wanted 20", loc.Col)
	}
	if loc.Scope.C != scope.C {
		t.Errorf("Got metadata %v as scope, though wanted %v", loc.Scope.C, scope.C)
	}
}

func TestIntrinsicBindings(t *testing.T) {
	ctx := NewContext()
	mod := ctx.NewModule("")
	defer mod.Dispose()

	memsetID := LookupIntrinsicID("llvm.memset")
	if memsetID == 0 {
		t.Fatal("could not look up llvm.memset intrinsic")
	}
	ptrTy := PointerType(ctx.Int8Type(), 0)
	fnTy := FunctionType(ctx.VoidType(), []Type{ptrTy}, false)
	fn := AddFunction(mod, "use_memset", fnTy)
	builder := ctx.NewBuilder()
	defer builder.Dispose()
	builder.SetInsertPointAtEnd(ctx.AddBasicBlock(fn, "entry"))
	call := builder.CreateIntrinsic(ctx.VoidType(), memsetID, []Value{
		fn.Param(0),
		ConstInt(ctx.Int8Type(), 0, false),
		ConstInt(ctx.Int64Type(), 8, false),
		ConstInt(ctx.Int1Type(), 0, false),
	}, "")
	builder.CreateRetVoid()
	if got := call.CalledValue().IntrinsicID(); got != memsetID {
		t.Fatalf("got intrinsic ID %d, want %d", got, memsetID)
	}
	if err := VerifyModule(mod, ReturnStatusAction); err != nil {
		t.Fatalf("module with intrinsic call should verify: %v", err)
	}
}

func TestConstTokenNoneWithCoroutineIntrinsics(t *testing.T) {
	ctx := NewContext()
	defer ctx.Dispose()

	none := ctx.ConstTokenNone()
	if none.IsNil() {
		t.Fatal("ConstTokenNone returned a nil value")
	}
	if got := none.Type(); got != ctx.TokenType() || got.TypeKind() != TokenTypeKind {
		t.Fatalf("ConstTokenNone type = %v (kind %v), want token", got, got.TypeKind())
	}
	if got := strings.TrimSpace(none.String()); got != "token none" {
		t.Fatalf("ConstTokenNone string = %q, want %q", got, "token none")
	}

	majorVersion, err := strconv.Atoi(strings.SplitN(Version, ".", 2)[0])
	if err != nil {
		t.Fatalf("could not parse LLVM version: %v", err)
	}

	mod := ctx.NewModule("coro-token-none")
	defer mod.Dispose()
	builder := ctx.NewBuilder()
	defer builder.Dispose()

	ptrTy := PointerType(ctx.Int8Type(), 0)
	fn := AddFunction(mod, "use_token_none", FunctionType(ctx.VoidType(), []Type{ptrTy}, false))
	builder.SetInsertPointAtEnd(ctx.AddBasicBlock(fn, "entry"))
	falseValue := ConstInt(ctx.Int1Type(), 0, false)

	suspendID := LookupIntrinsicID("llvm.coro.suspend")
	if suspendID == 0 {
		t.Fatal("could not look up llvm.coro.suspend intrinsic")
	}
	suspend := builder.CreateIntrinsic(ctx.Int8Type(), suspendID, []Value{none, falseValue}, "suspend")
	if suspend.IsNil() {
		t.Fatal("could not construct llvm.coro.suspend with token none")
	}

	// LLVM 18 added the unwind token operand to llvm.coro.end. LLVM 22
	// subsequently changed only its result type from i1 to void.
	if majorVersion >= 18 {
		endID := LookupIntrinsicID("llvm.coro.end")
		if endID == 0 {
			t.Fatal("could not look up llvm.coro.end intrinsic")
		}
		endType := ctx.Int1Type()
		endName := "end"
		if majorVersion >= 22 {
			endType = ctx.VoidType()
			endName = ""
		}
		end := builder.CreateIntrinsic(endType, endID, []Value{fn.Param(0), falseValue, none}, endName)
		if end.IsNil() {
			t.Fatal("could not construct llvm.coro.end with token none")
		}
	}
	builder.CreateRetVoid()

	if err := VerifyModule(mod, ReturnStatusAction); err != nil {
		t.Fatalf("module with token-none coroutine operands should verify: %v\n%s", err, mod.String())
	}
	text := mod.String()
	if !strings.Contains(text, "@llvm.coro.suspend(token none, i1 false)") {
		t.Fatalf("llvm.coro.suspend did not print token none:\n%s", text)
	}
	if majorVersion >= 18 && !strings.Contains(text, "i1 false, token none)") {
		t.Fatalf("llvm.coro.end did not print token none:\n%s", text)
	}
}

func TestSubtypes(t *testing.T) {
	cont := NewContext()
	defer cont.Dispose()

	st_pointer := cont.StructType([]Type{cont.Int32Type(), cont.Int8Type()}, false)
	st_inner := st_pointer.Subtypes()
	if len(st_inner) != 2 {
		t.Errorf("Got size %d, though wanted 2", len(st_inner))
	}
	if st_inner[0] != cont.Int32Type() {
		t.Errorf("Expected first struct field to be int32")
	}
	if st_inner[1] != cont.Int8Type() {
		t.Errorf("Expected second struct field to be int8")
	}
}

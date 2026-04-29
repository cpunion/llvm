//===- target_test.go - Tests for target bindings -------------------------===//
//
// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
//===----------------------------------------------------------------------===//

package llvm

import (
	"os"
	"testing"
)

func TestTargetMachineEmitToFile(t *testing.T) {
	InitializeNativeTarget()
	InitializeNativeAsmPrinter()

	triple := DefaultTargetTriple()
	target, err := GetTargetFromTriple(triple)
	if err != nil {
		t.Fatal(err)
	}
	tm := target.CreateTargetMachine(triple, "", "", CodeGenLevelDefault, RelocDefault, CodeModelDefault)
	if tm.C == nil {
		t.Fatal("CreateTargetMachine returned a nil target machine")
	}
	defer tm.Dispose()

	ctx := NewContext()
	mod := ctx.NewModule("emit_to_file_test")
	defer mod.Dispose()
	mod.SetTarget(triple)
	td := tm.CreateTargetData()
	defer td.Dispose()
	mod.SetDataLayout(td.String())

	fnType := FunctionType(ctx.Int32Type(), nil, false)
	fn := AddFunction(mod, "main", fnType)
	block := AddBasicBlock(fn, "entry")
	builder := ctx.NewBuilder()
	defer builder.Dispose()
	builder.SetInsertPointAtEnd(block)
	builder.CreateRet(ConstInt(ctx.Int32Type(), 0, false))

	if err := VerifyModule(mod, ReturnStatusAction); err != nil {
		t.Fatal(err)
	}
	obj, err := os.CreateTemp(t.TempDir(), "emit-*.o")
	if err != nil {
		t.Fatal(err)
	}
	objName := obj.Name()
	if err := obj.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tm.EmitToFile(mod, objName, ObjectFile); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(objName); err != nil {
		t.Fatal(err)
	} else if info.Size() == 0 {
		t.Fatal("EmitToFile produced an empty object file")
	}
}

func TestCreateTargetMachineWithOptionsSectionOptions(t *testing.T) {
	InitializeNativeTarget()

	triple := DefaultTargetTriple()
	target, err := GetTargetFromTriple(triple)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		opts TargetMachineOptions
	}{
		{
			name: "zero value",
			opts: TargetMachineOptions{},
		},
		{
			name: "mixed flags",
			opts: TargetMachineOptions{
				FunctionSections:   true,
				DataSections:       false,
				UniqueSectionNames: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tm := target.CreateTargetMachineWithOptions(
				triple, "", "", CodeGenLevelDefault, RelocDefault, CodeModelDefault, tc.opts,
			)
			if tm.C == nil {
				t.Fatal("CreateTargetMachineWithOptions returned a nil target machine")
			}
			defer tm.Dispose()

			if got := tm.sectionOptions(); got != tc.opts {
				t.Fatalf("section options mismatch: got %+v, want %+v", got, tc.opts)
			}
		})
	}
}

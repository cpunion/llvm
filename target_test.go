//===- target_test.go - Tests for target bindings -------------------------===//
//
// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
//===----------------------------------------------------------------------===//

package llvm

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"strings"
	"testing"
)

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

func TestCreateTargetMachineWithOptionsRISCVABIName(t *testing.T) {
	InitializeAllTargetInfos()
	InitializeAllTargets()
	InitializeAllTargetMCs()
	InitializeAllAsmPrinters()

	const (
		triple              = "riscv64-unknown-elf"
		features            = "+m,+a,+f,+d,+c"
		riscvFloatABIMask   = uint32(0x6)
		riscvFloatABIDouble = uint32(0x4)
	)
	target, err := GetTargetFromTriple(triple)
	if err != nil {
		t.Skipf("RISC-V target not available: %v", err)
	}

	tests := []struct {
		abi          string
		wantELFFlags uint32
		wantFA0      bool
	}{
		{abi: "lp64", wantELFFlags: 0, wantFA0: false},
		{abi: "lp64d", wantELFFlags: riscvFloatABIDouble, wantFA0: true},
	}
	for _, tc := range tests {
		t.Run(tc.abi, func(t *testing.T) {
			tm := target.CreateTargetMachineWithOptions(
				triple, "generic-rv64", features,
				CodeGenLevelNone, RelocDefault, CodeModelDefault,
				TargetMachineOptions{ABIName: tc.abi},
			)
			if tm.C == nil {
				t.Fatal("CreateTargetMachineWithOptions returned a nil target machine")
			}
			defer tm.Dispose()

			ctx := NewContext()
			defer ctx.Dispose()
			mod := ctx.NewModule("riscv_abi_name_test")
			defer mod.Dispose()
			mod.SetTarget(triple)
			td := tm.CreateTargetData()
			defer td.Dispose()
			mod.SetDataLayout(td.String())

			calleeType := FunctionType(ctx.VoidType(), []Type{ctx.DoubleType()}, false)
			callee := AddFunction(mod, "callee", calleeType)
			callerType := FunctionType(ctx.VoidType(), nil, false)
			caller := AddFunction(mod, "caller", callerType)
			entry := AddBasicBlock(caller, "entry")
			builder := ctx.NewBuilder()
			defer builder.Dispose()
			builder.SetInsertPointAtEnd(entry)
			builder.CreateCall(
				calleeType, callee, []Value{ConstFloat(ctx.DoubleType(), 1.25)}, "",
			)
			builder.CreateRetVoid()

			if err := VerifyModule(mod, ReturnStatusAction); err != nil {
				t.Fatal(err)
			}

			object, err := tm.EmitToMemoryBuffer(mod, ObjectFile)
			if err != nil {
				t.Fatal(err)
			}
			defer object.Dispose()
			flags := elfFlags(t, object.Bytes())
			if got := flags & riscvFloatABIMask; got != tc.wantELFFlags {
				t.Fatalf("RISC-V ELF float ABI flags = %#x, want %#x (all flags %#x)", got, tc.wantELFFlags, flags)
			}

			assembly, err := tm.EmitToMemoryBuffer(mod, AssemblyFile)
			if err != nil {
				t.Fatal(err)
			}
			defer assembly.Dispose()
			asm := string(assembly.Bytes())
			if got := strings.Contains(asm, "fa0"); got != tc.wantFA0 {
				t.Fatalf("assembly fa0 presence = %v, want %v:\n%s", got, tc.wantFA0, asm)
			}
		})
	}
}

func elfFlags(t *testing.T, object []byte) uint32 {
	t.Helper()
	file, err := elf.NewFile(bytes.NewReader(object))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if file.Machine != elf.EM_RISCV {
		t.Fatalf("ELF machine = %v, want %v", file.Machine, elf.EM_RISCV)
	}

	reader := bytes.NewReader(object)
	switch file.Class {
	case elf.ELFCLASS32:
		var header elf.Header32
		if err := binary.Read(reader, file.ByteOrder, &header); err != nil {
			t.Fatal(err)
		}
		return header.Flags
	case elf.ELFCLASS64:
		var header elf.Header64
		if err := binary.Read(reader, file.ByteOrder, &header); err != nil {
			t.Fatal(err)
		}
		return header.Flags
	default:
		t.Fatalf("unsupported ELF class %v", file.Class)
		return 0
	}
}

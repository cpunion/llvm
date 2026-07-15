//===- coroutine_test.go - Tests for coroutine lowering ------------------===//
//
// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
//===----------------------------------------------------------------------===//

package llvm

import (
	"strconv"
	"strings"
	"testing"
)

func TestSwitchedResumeCoroutinePassPipelines(t *testing.T) {
	majorVersion, err := strconv.Atoi(strings.SplitN(Version, ".", 2)[0])
	if err != nil {
		t.Fatalf("could not parse LLVM version %q: %v", Version, err)
	}

	InitializeAllTargetInfos()
	InitializeAllTargets()
	InitializeAllTargetMCs()
	InitializeAllAsmPrinters()
	triple := DefaultTargetTriple()
	target, err := GetTargetFromTriple(triple)
	if err != nil {
		t.Fatal(err)
	}
	tm := target.CreateTargetMachine(
		triple, "", "", CodeGenLevelDefault, RelocDefault, CodeModelDefault,
	)
	defer tm.Dispose()

	explicitPipeline := "coro-early,cgscc(coro-split),coro-cleanup"
	if majorVersion == 14 {
		// In LLVM 14, coro-early and coro-cleanup are function passes. Newer
		// releases expose them as module passes and accept the unnested form.
		explicitPipeline = "function(coro-early),cgscc(coro-split),function(coro-cleanup)"
	}
	for _, test := range []struct {
		name     string
		pipeline string
	}{
		{name: "explicit coroutine pipeline", pipeline: explicitPipeline},
		{name: "production O0 pipeline", pipeline: "default<O0>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := NewContext()
			defer ctx.Dispose()
			mod := buildSwitchedResumeCoroutine(t, ctx, tm, majorVersion)
			defer mod.Dispose()
			assertSwitchedResumeCoroutineFrontendState(t, mod, majorVersion)

			if err := VerifyModule(mod, ReturnStatusAction); err != nil {
				t.Fatalf("coroutine module did not verify before %s: %v\n%s", test.pipeline, err, mod.String())
			}

			options := NewPassBuilderOptions()
			defer options.Dispose()
			options.SetVerifyEach(true)
			if err := mod.RunPasses(test.pipeline, tm, options); err != nil {
				t.Fatalf("could not run %s: %v\n%s", test.pipeline, err, mod.String())
			}

			if err := VerifyModule(mod, ReturnStatusAction); err != nil {
				t.Fatalf("coroutine module did not verify after %s: %v\n%s", test.pipeline, err, mod.String())
			}
			assertSwitchedResumeCoroutineLowered(t, mod, test.pipeline)
		})
	}
}

func buildSwitchedResumeCoroutine(t *testing.T, ctx Context, tm TargetMachine, majorVersion int) Module {
	t.Helper()

	mod := ctx.NewModule("switched-resume-coroutine")
	returned := false
	defer func() {
		if !returned {
			mod.Dispose()
		}
	}()
	mod.SetTarget(tm.Triple())
	targetData := tm.CreateTargetData()
	mod.SetDataLayout(targetData.String())
	pointerSize := targetData.PointerSize()
	targetData.Dispose()

	i1 := ctx.Int1Type()
	i8 := ctx.Int8Type()
	i32 := ctx.Int32Type()
	ptr := PointerType(i8, 0)
	void := ctx.VoidType()
	falseValue := ConstInt(i1, 0, false)
	nullPointer := ConstPointerNull(ptr)
	var sizeType Type
	switch pointerSize {
	case 4:
		sizeType = i32
	case 8:
		sizeType = ctx.Int64Type()
	default:
		t.Fatalf("unsupported target pointer size %d", pointerSize)
	}

	fn := AddFunction(mod, "stackless", FunctionType(ptr, nil, false))
	if majorVersion == 14 {
		// LLVM 14 models this as a frontend-to-CoroEarly state machine: "0"
		// means unprepared input, while "1" is reserved for prepared IR.
		fn.AddFunctionAttr(ctx.CreateStringAttribute("coroutine.presplit", "0"))
	} else {
		kind := AttributeKindID("presplitcoroutine")
		if kind == 0 {
			t.Fatal("could not look up presplitcoroutine attribute")
		}
		fn.AddFunctionAttr(ctx.CreateEnumAttribute(kind, 0))
	}

	frameAllocType := FunctionType(ptr, []Type{sizeType, sizeType}, false)
	frameAlloc := AddFunction(mod, "coro_frame_alloc", frameAllocType)
	freeType := FunctionType(void, []Type{ptr}, false)
	free := AddFunction(mod, "coro_frame_free", freeType)

	entry := AddBasicBlock(fn, "entry")
	allocate := AddBasicBlock(fn, "allocate")
	begin := AddBasicBlock(fn, "begin")
	initialSuspend := AddBasicBlock(fn, "initial.suspend")
	body := AddBasicBlock(fn, "body")
	finalSuspend := AddBasicBlock(fn, "final.suspend")
	finalResumeUnreachable := AddBasicBlock(fn, "final.resume.unreachable")
	cleanup := AddBasicBlock(fn, "cleanup")
	deallocate := AddBasicBlock(fn, "deallocate")
	end := AddBasicBlock(fn, "end")

	ids := make(map[string]int)
	for _, name := range []string{
		"llvm.coro.id",
		"llvm.coro.alloc",
		"llvm.coro.size",
		"llvm.coro.align",
		"llvm.coro.begin",
		"llvm.coro.save",
		"llvm.coro.suspend",
		"llvm.coro.free",
		"llvm.coro.end",
	} {
		ids[name] = LookupIntrinsicID(name)
		if ids[name] == 0 {
			t.Fatalf("could not look up %s intrinsic", name)
		}
	}

	builder := ctx.NewBuilder()
	defer builder.Dispose()

	createIntrinsic := func(ret Type, name string, args []Value, resultName string) Value {
		value := builder.CreateIntrinsic(ret, ids[name], args, resultName)
		if value.IsNil() {
			t.Fatalf("could not construct %s intrinsic", name)
		}
		return value
	}
	createSuspend := func(block BasicBlock, handle Value, final bool, resume, destroy BasicBlock) {
		builder.SetInsertPointAtEnd(block)
		saved := createIntrinsic(ctx.TokenType(), "llvm.coro.save", []Value{handle}, "save")
		finalValue := ConstInt(i1, 0, false)
		if final {
			finalValue = ConstInt(i1, 1, false)
		}
		suspended := createIntrinsic(i8, "llvm.coro.suspend", []Value{saved, finalValue}, "suspend")
		dispatch := builder.CreateSwitch(suspended, end, 2)
		dispatch.AddCase(ConstInt(i8, 0, false), resume)
		dispatch.AddCase(ConstInt(i8, 1, false), destroy)
	}

	builder.SetInsertPointAtEnd(entry)
	// Zero requests LLVM's default allocation memory alignment guarantee of
	// twice the target pointer size.
	allocationAlignmentGuarantee := ConstInt(i32, 0, false)
	id := createIntrinsic(ctx.TokenType(), "llvm.coro.id", []Value{
		allocationAlignmentGuarantee,
		nullPointer,
		nullPointer,
		nullPointer,
	}, "id")
	needsAllocation := createIntrinsic(i1, "llvm.coro.alloc", []Value{id}, "needs.alloc")
	builder.CreateCondBr(needsAllocation, allocate, begin)

	builder.SetInsertPointAtEnd(allocate)
	frameSize := createIntrinsic(sizeType, "llvm.coro.size", nil, "frame.size")
	frameAlignment := createIntrinsic(sizeType, "llvm.coro.align", nil, "frame.align")
	minimumAlignment := ConstInt(sizeType, uint64(2*pointerSize), false)
	belowMinimum := builder.CreateICmp(IntULT, frameAlignment, minimumAlignment, "below.minimum.alignment")
	allocationAlignment := builder.CreateSelect(
		belowMinimum, minimumAlignment, frameAlignment, "allocation.alignment",
	)
	allocatedMemory := builder.CreateCall(
		frameAllocType, frameAlloc, []Value{frameSize, allocationAlignment}, "allocated.memory",
	)
	builder.CreateBr(begin)

	builder.SetInsertPointAtEnd(begin)
	frameMemory := builder.CreatePHI(ptr, "frame.memory")
	frameMemory.AddIncoming(
		[]Value{nullPointer, allocatedMemory},
		[]BasicBlock{entry, allocate},
	)
	handle := createIntrinsic(ptr, "llvm.coro.begin", []Value{id, frameMemory}, "handle")
	builder.CreateBr(initialSuspend)

	createSuspend(initialSuspend, handle, false, body, cleanup)
	createSuspend(body, handle, false, finalSuspend, cleanup)
	createSuspend(finalSuspend, handle, true, finalResumeUnreachable, cleanup)

	builder.SetInsertPointAtEnd(finalResumeUnreachable)
	builder.CreateUnreachable()

	builder.SetInsertPointAtEnd(cleanup)
	frameToFree := createIntrinsic(ptr, "llvm.coro.free", []Value{id, handle}, "frame.to.free")
	hasFrameToFree := builder.CreateIsNotNull(frameToFree, "has.frame.to.free")
	builder.CreateCondBr(hasFrameToFree, deallocate, end)

	builder.SetInsertPointAtEnd(deallocate)
	builder.CreateCall(freeType, free, []Value{frameToFree}, "")
	builder.CreateBr(end)

	builder.SetInsertPointAtEnd(end)
	endArgs := []Value{handle, falseValue}
	endType := i1
	endName := "ended"
	if majorVersion >= 18 {
		endArgs = append(endArgs, ctx.ConstTokenNone())
	}
	if majorVersion >= 22 {
		endType = void
		endName = ""
	}
	createIntrinsic(endType, "llvm.coro.end", endArgs, endName)
	builder.CreateRet(handle)

	returned = true
	return mod
}

func assertSwitchedResumeCoroutineFrontendState(t *testing.T, mod Module, majorVersion int) {
	t.Helper()

	text := mod.String()
	if majorVersion == 14 {
		if !strings.Contains(text, `"coroutine.presplit"="0"`) {
			t.Fatalf("LLVM 14 frontend coroutine is not marked unprepared for CoroEarly:\n%s", text)
		}
	} else if !strings.Contains(text, "presplitcoroutine") {
		t.Fatalf("frontend coroutine lacks presplitcoroutine attribute:\n%s", text)
	}
	if !strings.Contains(text, "%allocation.alignment = select i1") {
		t.Fatalf("frame allocator does not normalize coro.align to the coro.id guarantee:\n%s", text)
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "call") && strings.Contains(line, "@coro_frame_alloc") {
			if !strings.Contains(line, "%allocation.alignment") {
				t.Fatalf("frame allocator does not receive the normalized alignment: %s\n%s", line, text)
			}
			return
		}
	}
	t.Fatalf("frontend coroutine does not call the frame allocator:\n%s", text)
}

func assertSwitchedResumeCoroutineLowered(t *testing.T, mod Module, pipeline string) {
	t.Helper()

	for _, name := range []string{"stackless", "stackless.resume", "stackless.destroy"} {
		fn := mod.NamedFunction(name)
		if fn.IsNil() || fn.BasicBlocksCount() == 0 {
			t.Fatalf("%s did not produce coroutine function %q:\n%s", pipeline, name, mod.String())
		}
	}

	text := mod.String()
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, " call ") && strings.Contains(line, "@llvm.coro.") {
			t.Fatalf("%s left coroutine intrinsic call after lowering: %s\n%s", pipeline, line, text)
		}
	}
}

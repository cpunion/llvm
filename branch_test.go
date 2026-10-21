package llvm

import "testing"

func TestBranchOpcodeCompatibility(t *testing.T) {
	ctx := NewContext()
	defer ctx.Dispose()
	mod := ctx.NewModule("branches")
	defer mod.Dispose()
	b := ctx.NewBuilder()
	defer b.Dispose()
	fn := AddFunction(mod, "branch", FunctionType(ctx.VoidType(), []Type{ctx.Int1Type()}, false))
	entry := ctx.AddBasicBlock(fn, "entry")
	left := ctx.AddBasicBlock(fn, "left")
	right := ctx.AddBasicBlock(fn, "right")
	b.SetInsertPointAtEnd(entry)
	conditional := b.CreateCondBr(fn.Param(0), left, right)
	b.SetInsertPointAtEnd(left)
	unconditional := b.CreateBr(right)
	b.SetInsertPointAtEnd(right)
	ret := b.CreateRetVoid()
	for _, tc := range []struct {
		name       string
		branch     Value
		successors int
	}{{"conditional", conditional, 2}, {"unconditional", unconditional, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.branch.InstructionOpcode() != Br || tc.branch.IsABranchInst() != tc.branch {
				t.Fatal("shared branch classification changed", tc.branch.String())
			}
			if tc.branch.SuccessorsCount() != tc.successors {
				t.Fatal("branch control flow changed", tc.branch.String())
			}
		})
	}
	if ret.InstructionOpcode() != Ret || !ret.IsABranchInst().IsNil() {
		t.Fatal("non-branch instruction was classified as a branch")
	}
	if err := VerifyModule(mod, ReturnStatusAction); err != nil {
		t.Fatal(err)
	}
}

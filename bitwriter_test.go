package llvm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFullLTOBitcodeToMemoryBufferAddsFullLTOFlag(t *testing.T) {
	ctx := NewContext()
	defer ctx.Dispose()
	mod := ctx.NewModule("full_lto")
	defer mod.Dispose()

	fnTy := FunctionType(ctx.Int32Type(), nil, false)
	fn := AddFunction(mod, "main", fnTy)
	b := ctx.NewBuilder()
	defer b.Dispose()
	b.SetInsertPointAtEnd(ctx.AddBasicBlock(fn, "entry"))
	b.CreateRet(ConstInt(ctx.Int32Type(), 0, false))

	buf := WriteFullLTOBitcodeToMemoryBuffer(mod, true)
	defer buf.Dispose()
	verifyLTOBitcodeDefinition(t, ctx, buf)

	ir := mod.String()
	if !strings.Contains(ir, `i32 1, !"ThinLTO", i32 0`) {
		t.Fatalf("missing full LTO ThinLTO module flag:\n%s", ir)
	}
	if strings.Contains(ir, `"EnableSplitLTOUnit"`) {
		t.Fatalf("split-unit state should be controlled by the summary index, not a module flag:\n%s", ir)
	}
}

func TestWriteThinLTOBitcodeWithDefinition(t *testing.T) {
	mod := transformationModule(t, `define i32 @main() { ret i32 0 }`)
	buf := WriteThinLTOBitcodeToMemoryBuffer(mod)
	defer buf.Dispose()
	verifyLTOBitcodeDefinition(t, mod.Context(), buf)
}

func verifyLTOBitcodeDefinition(t *testing.T, ctx Context, buf MemoryBuffer) {
	t.Helper()
	if buf.IsNil() {
		t.Fatal("LTO writer returned nil buffer")
	}
	path := filepath.Join(t.TempDir(), "lto.bc")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	roundTrip, err := ctx.ParseBitcodeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer roundTrip.Dispose()
	if err := VerifyModule(roundTrip, ReturnStatusAction); err != nil {
		t.Fatal(err)
	}
	if fn := roundTrip.NamedFunction("main"); fn.IsNil() || fn.IsDeclaration() {
		t.Fatal("function definition was lost in LTO bitcode")
	}
}

package llvm

import (
	"fmt"
	"testing"
)

func TestConstIntTruncatesAndUniques(t *testing.T) {
	ctx := NewContext()
	defer ctx.Dispose()
	for _, width := range []int{1, 8, 16, 32, 64} {
		for _, raw := range []uint64{0, 1, 128, 255, 256, 257, 1 << 63, ^uint64(0)} {
			for _, signed := range []bool{false, true} {
				t.Run(fmt.Sprintf("i%d/%x/signed=%t", width, raw, signed), func(t *testing.T) {
					ty := ctx.IntType(width)
					want := raw
					if width < 64 {
						want &= (uint64(1) << width) - 1
					}
					got := ConstInt(ty, raw, signed)
					if got.ZExtValue() != want {
						t.Fatalf("constant = %#x, want %#x", got.ZExtValue(), want)
					}
					// Equal bit patterns must be the same LLVM constant, even
					// when the original input contained discarded high bits.
					canonical := ConstInt(ty, want, false)
					if got != canonical {
						t.Fatalf("constant was not canonicalized: %s != %s", got, canonical)
					}
				})
			}
		}
	}
	wide := ctx.IntType(128)
	for _, signed := range []bool{false, true} {
		text := "18446744073709551615"
		if signed {
			text = "-1"
		}
		if got, want := ConstInt(wide, ^uint64(0), signed), ConstIntFromString(wide, text, 10); got != want {
			t.Fatalf("i128 signed=%t: %s != %s", signed, got, want)
		}
	}
}

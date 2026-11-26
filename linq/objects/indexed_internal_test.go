package objects

import (
	"math"
	"testing"
)

func TestAdvanceIndexAllowsMaxIntAndPanicsNext(t *testing.T) {
	if first := advanceIndex(-1); first != 0 {
		t.Errorf("first index = %d, want 0", first)
	}
	last := advanceIndex(math.MaxInt - 1)
	if last != math.MaxInt {
		t.Fatalf("last index = %d, want %d", last, math.MaxInt)
	}
	defer func() {
		if got := recover(); got != "objects: index exceeds int capacity" {
			t.Errorf("overflow panic = %v", got)
		}
	}()
	advanceIndex(last)
}

package objects_test

import (
	"math"
	"testing"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func TestCountPanicsOnOverflowAndCleansUp(t *testing.T) {
	// A 32-bit Go int makes it practical to reach the public overflow boundary.
	var pulled uint32
	cleaned := false
	source := func(yield func(struct{}) bool) {
		defer func() { cleaned = true }()
		for pulled <= math.MaxInt {
			pulled++
			if !yield(struct{}{}) {
				return
			}
		}
	}
	defer func() {
		if p := recover(); p != "objects: count exceeds int capacity" {
			t.Errorf("overflow panic = %v, want count overflow", p)
		}
		if pulled != uint32(math.MaxInt)+1 || !cleaned {
			t.Errorf("pulled %d values and cleaned up %t, want %d and true", pulled, cleaned, uint32(math.MaxInt)+1)
		}
	}()
	objects.Count(source)
}

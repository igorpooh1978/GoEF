package objects_test

import (
	"iter"
	"math"
	"testing"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func TestIndexedOperatorsOverflowReleasesSource(t *testing.T) {
	for _, mode := range []struct {
		name string
		run  func(iter.Seq[struct{}], func(int))
	}{
		{"where", func(source iter.Seq[struct{}], observe func(int)) {
			for range objects.WhereIndexed(source, func(_ struct{}, index int) bool {
				observe(index)
				return false
			}) {
				panic("predicate should have rejected the value")
			}
		}},
		{"select", func(source iter.Seq[struct{}], observe func(int)) {
			for range objects.SelectIndexed(source, func(_ struct{}, index int) struct{} {
				observe(index)
				return struct{}{}
			}) {
			}
		}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			var pulls uint64
			cleaned := false
			source := func(yield func(struct{}) bool) {
				defer func() { cleaned = true }()
				for {
					pulls++
					if !yield(struct{}{}) {
						return
					}
				}
			}
			last := -1
			defer func() {
				if got := recover(); got != "objects: index exceeds int capacity" {
					t.Errorf("overflow panic = %v", got)
				}
				if !cleaned || last != math.MaxInt || pulls != uint64(math.MaxInt)+2 {
					t.Errorf("last callback index = %d, pulls = %d, cleaned = %t", last, pulls, cleaned)
				}
			}()
			mode.run(source, func(index int) { last = index })
		})
	}
}

package optional_test

import (
	"testing"

	"github.com/imbrooklyn/shuttle/optional"
)

var optionalBenchmarkSink optional.Optional[int]

func BenchmarkOptionalTransformations(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		optionalBenchmarkSink = optional.Some(10).
			Map(func(value int) int { return value + 1 }).
			FlatMap(func(value int) optional.Optional[int] { return optional.Some(value * 2) }).
			Filter(func(value int) bool { return value > 0 }).
			Inspect(func(int) {})
	}
}

func TestOptionalTransformationsDoNotAllocate(t *testing.T) {
	allocations := testing.AllocsPerRun(1_000, func() {
		optionalBenchmarkSink = optional.Some(10).
			Map(func(value int) int { return value + 1 }).
			FlatMap(func(value int) optional.Optional[int] { return optional.Some(value * 2) }).
			Filter(func(value int) bool { return value > 0 }).
			Inspect(func(int) {})
	})
	if allocations != 0 {
		t.Fatalf("transformations allocated %.2f times per run, want 0", allocations)
	}
}

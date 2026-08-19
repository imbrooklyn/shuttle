package stream_test

import (
	"testing"

	"github.com/imbrooklyn/shuttle/stream"
)

var allocationSink int

func incrementalAllocations(values []int) float64 {
	return testing.AllocsPerRun(100, func() {
		pipeline := stream.FromSlice(values).
			Map(func(value int) int { return value + 1 }).
			Filter(func(value int) bool { return value > 0 }).
			Inspect(func(int) {}).
			Take(len(values)).
			Skip(0).
			TakeWhile(func(int) bool { return true }).
			SkipWhile(func(int) bool { return false })
		pipeline.ForEach(func(value int) { allocationSink += value })
	})
}

func TestIncrementalOperatorAllocationsDoNotScalePerElement(t *testing.T) {
	small := incrementalAllocations(make([]int, 10))
	large := incrementalAllocations(make([]int, 10_000))
	if large > small+2 {
		t.Fatalf("allocations scaled with elements: 10 elements %.2f, 10K elements %.2f", small, large)
	}
}

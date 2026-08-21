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

func flatMapSliceAllocations(outerCount, innerCount int) float64 {
	inner := make([]int, innerCount)
	for index := range inner {
		inner[index] = index + 1
	}
	outer := make([][]int, outerCount)
	for index := range outer {
		outer[index] = inner
	}
	pipeline := stream.FromSlice(outer).FlatMapSlice(func(values []int) []int { return values })
	return testing.AllocsPerRun(100, func() {
		pipeline.ForEach(func(value int) { allocationSink += value })
	})
}

func TestFlatMapSliceAllocationsDoNotScaleWithOuterOrInnerElements(t *testing.T) {
	fewOuter := flatMapSliceAllocations(10, 1)
	manyOuter := flatMapSliceAllocations(10_000, 1)
	if manyOuter > fewOuter+2 {
		t.Fatalf("allocations scaled with outer elements: 10 %.2f, 10K %.2f", fewOuter, manyOuter)
	}

	fewInner := flatMapSliceAllocations(1, 10)
	manyInner := flatMapSliceAllocations(1, 10_000)
	if manyInner > fewInner+2 {
		t.Fatalf("allocations scaled with inner elements: 10 %.2f, 10K %.2f", fewInner, manyInner)
	}
}

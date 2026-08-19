package stream_test

import (
	"iter"
	"math"
	"slices"
	"testing"

	"github.com/imbrooklyn/shuttle/stream"
)

func TestZeroEmptyAndSeq(t *testing.T) {
	var zero stream.Stream[int]
	for name, value := range map[string]stream.Stream[int]{
		"zero":  zero,
		"Empty": stream.Empty[int](),
	} {
		t.Run(name, func(t *testing.T) {
			seq := value.Seq()
			if seq == nil {
				t.Fatal("Seq returned nil")
			}
			if got := value.Collect(); got != nil {
				t.Fatalf("Collect = %v, want nil", got)
			}
			if got := value.Count(); got != 0 {
				t.Fatalf("Count = %d, want 0", got)
			}
		})
	}
}

func TestLargeSource(t *testing.T) {
	if got := stream.Range(0, 100_000).Count(); got != 100_000 {
		t.Fatalf("large Count = %d, want 100000", got)
	}
}

func TestOfSnapshotsAndFromSliceViews(t *testing.T) {
	values := []int{1, 2, 3}
	snapshot := stream.Of(values...)
	view := stream.FromSlice(values)
	values[1] = 9
	values = values[:1]
	if len(values) != 1 {
		t.Fatal("caller reslice failed")
	}

	requireSliceEqual(t, snapshot.Collect(), []int{1, 2, 3})
	requireSliceEqual(t, view.Collect(), []int{1, 9, 3})
	requireSliceEqual(t, snapshot.Collect(), []int{1, 2, 3})
	requireSliceEqual(t, view.Collect(), []int{1, 9, 3})

	if got := stream.Of[int]().Collect(); got != nil {
		t.Fatalf("Of with no values = %v, want nil", got)
	}
	if got := stream.FromSlice[int](nil).Collect(); got != nil {
		t.Fatalf("FromSlice(nil) = %v, want nil", got)
	}

	referenced := [][]int{{1}}
	shallowSnapshot := stream.Of(referenced...)
	referenced[0][0] = 7
	referenced[0] = []int{9}
	if got := shallowSnapshot.First().Must()[0]; got != 7 {
		t.Fatalf("Of did not preserve shallow aliasing: got %d, want 7", got)
	}
}

func TestFromSeqPreservesProtocolAndSingleUse(t *testing.T) {
	if got := stream.FromSeq[int](nil).Collect(); got != nil {
		t.Fatalf("FromSeq(nil) = %v, want nil", got)
	}

	probe := new(sequenceProbe)
	wrapped := stream.FromSeq(instrumentedSeq([]int{1, 2, 3}, probe))
	if got := wrapped.First().Must(); got != 1 {
		t.Fatalf("First = %d", got)
	}
	if probe.consumed.Load() != 1 || probe.cleanups.Load() != 1 {
		t.Fatalf("probe after First = consumed %d cleanup %d", probe.consumed.Load(), probe.cleanups.Load())
	}

	index := 0
	singleUse := stream.FromSeq(func(yield func(int) bool) {
		for index < 3 {
			index++
			if !yield(index) {
				return
			}
		}
	})
	copyOfDescriptor := singleUse
	if got := singleUse.First().Must(); got != 1 {
		t.Fatalf("single-use First = %d", got)
	}
	requireSliceEqual(t, copyOfDescriptor.Collect(), []int{2, 3})
}

func TestSeq2Interoperability(t *testing.T) {
	if got := stream.FromSeq2[string, int](nil).Collect(); got != nil {
		t.Fatalf("FromSeq2(nil) = %v, want nil", got)
	}

	called := 0
	seq2 := iter.Seq2[string, int](func(yield func(string, int) bool) {
		for _, pair := range []stream.Pair[string, int]{{First: "a", Second: 1}, {First: "b", Second: 2}} {
			called++
			if !yield(pair.First, pair.Second) {
				return
			}
		}
	})
	pairs := stream.FromSeq2(seq2)
	if got := pairs.First().Must(); got != (stream.Pair[string, int]{First: "a", Second: 1}) {
		t.Fatalf("First pair = %#v", got)
	}
	if called != 1 {
		t.Fatalf("FromSeq2 did not propagate false: calls = %d", called)
	}

	var roundTrip []stream.Pair[string, int]
	stream.ToSeq2(stream.Of(
		stream.Pair[string, int]{First: "x", Second: 7},
		stream.Pair[string, int]{First: "y", Second: 8},
	))(func(a string, b int) bool {
		roundTrip = append(roundTrip, stream.Pair[string, int]{First: a, Second: b})
		return true
	})
	if !slices.Equal(roundTrip, []stream.Pair[string, int]{{First: "x", Second: 7}, {First: "y", Second: 8}}) {
		t.Fatalf("Seq2 round trip = %v", roundTrip)
	}
}

func TestRangeAndRangeStep(t *testing.T) {
	tests := []struct {
		name             string
		start, end, step int
		want             []int
	}{
		{name: "ascending", start: 1, end: 6, step: 2, want: []int{1, 3, 5}},
		{name: "descending", start: 5, end: 0, step: -2, want: []int{5, 3, 1}},
		{name: "positive wrong direction", start: 5, end: 0, step: 1},
		{name: "negative wrong direction", start: 0, end: 5, step: -1},
		{name: "equal bounds", start: 2, end: 2, step: 1},
		{name: "positive overflow boundary", start: math.MaxInt - 1, end: math.MaxInt, step: 2, want: []int{math.MaxInt - 1}},
		{name: "negative overflow boundary", start: math.MinInt + 1, end: math.MinInt, step: -2, want: []int{math.MinInt + 1}},
		{name: "minimum step", start: 0, end: math.MinInt, step: math.MinInt, want: []int{0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireSliceEqual(t, stream.RangeStep(test.start, test.end, test.step).Collect(), test.want)
		})
	}
	requireSliceEqual(t, stream.Range(2, 5).Collect(), []int{2, 3, 4})
	if got := stream.RangeStep(1, 10, 2).First().Must(); got != 1 {
		t.Fatalf("RangeStep First = %d", got)
	}
	if got := stream.Range(5, 2).Collect(); got != nil {
		t.Fatalf("descending Range = %v, want nil", got)
	}
	requirePanics(t, "RangeStep zero", func() { stream.RangeStep(0, 1, 0) })
}

func TestRepeatIterateAndGenerate(t *testing.T) {
	requireSliceEqual(t, stream.Repeat("x").Take(3).Collect(), []string{"x", "x", "x"})
	requireSliceEqual(t, stream.RepeatN(4, 3).Collect(), []int{4, 4, 4})
	if got := stream.RepeatN(4, 3).First().Must(); got != 4 {
		t.Fatalf("RepeatN First = %d", got)
	}
	if got := stream.RepeatN(4, 0).Collect(); got != nil {
		t.Fatalf("RepeatN zero = %v, want nil", got)
	}
	requirePanics(t, "RepeatN negative", func() { stream.RepeatN(1, -1) })

	referenceValue := []int{1}
	repeatedReferences := stream.Repeat(referenceValue).Take(2).Collect()
	repeatedReferences[0][0] = 8
	if repeatedReferences[1][0] != 8 || referenceValue[0] != 8 {
		t.Fatalf("Repeat did not preserve shallow aliases: repeated=%v source=%v", repeatedReferences, referenceValue)
	}

	nextCalls := 0
	iterated := stream.Iterate(1, func(value int) int {
		nextCalls++
		return value * 2
	})
	requireSliceEqual(t, iterated.Take(1).Collect(), []int{1})
	if nextCalls != 0 {
		t.Fatalf("Iterate next calls after Take(1) = %d", nextCalls)
	}
	requireSliceEqual(t, iterated.Take(3).Collect(), []int{1, 2, 4})
	if nextCalls != 2 {
		t.Fatalf("Iterate next calls = %d, want 2", nextCalls)
	}

	generated := 0
	generator := stream.Generate(func() int {
		generated++
		return generated
	})
	if got := generator.Take(0).Collect(); got != nil || generated != 0 {
		t.Fatalf("Generate Take(0) = %v, calls = %d", got, generated)
	}
	if got := generator.First().Must(); got != 1 {
		t.Fatalf("first generated value = %d", got)
	}
	if got := generator.First().Must(); got != 2 {
		t.Fatalf("shared generator value = %d", got)
	}
}

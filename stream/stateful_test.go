package stream_test

import (
	"cmp"
	"math"
	"slices"
	"testing"

	"github.com/imbrooklyn/shuttle/stream"
)

func TestDistinctAndDistinctBy(t *testing.T) {
	keyCalls := 0
	distinct := stream.Of("one", "two", "three", "four", "six").DistinctBy(func(value string) int {
		keyCalls++
		return len(value)
	})
	requireSliceEqual(t, distinct.Collect(), []string{"one", "three", "four"})
	if keyCalls != 5 {
		t.Fatalf("key calls = %d, want 5", keyCalls)
	}
	requireSliceEqual(t, distinct.Collect(), []string{"one", "three", "four"})
	if keyCalls != 10 {
		t.Fatalf("second traversal key calls = %d, want 10", keyCalls)
	}
	requireSliceEqual(t, stream.Distinct(stream.Of(3, 1, 3, 2, 1)).Collect(), []int{3, 1, 2})

	requirePanics(t, "Distinct dynamic non-comparable key", func() {
		stream.Distinct(stream.Of[any]([]int{1})).Collect()
	})
	requirePanics(t, "DistinctBy dynamic non-comparable key", func() {
		stream.Of(1).DistinctBy(func(int) any { return []int{1} }).Collect()
	})
}

func TestChunkBoundariesAndOwnership(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		size  int
		want  [][]int
	}{
		{name: "empty", size: 3},
		{name: "partial", input: []int{1, 2}, size: 3, want: [][]int{{1, 2}}},
		{name: "exact", input: []int{1, 2, 3}, size: 3, want: [][]int{{1, 2, 3}}},
		{name: "full and partial", input: []int{1, 2, 3, 4, 5, 6, 7}, size: 3, want: [][]int{{1, 2, 3}, {4, 5, 6}, {7}}},
		{name: "singletons", input: []int{1, 2}, size: 1, want: [][]int{{1}, {2}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := stream.Chunk(stream.FromSlice(test.input), test.size).Collect()
			if !slices.EqualFunc(got, test.want, slices.Equal[[]int]) {
				t.Fatalf("Chunk = %v, want %v", got, test.want)
			}
			for _, chunk := range got {
				if cap(chunk) != len(chunk) {
					t.Fatalf("chunk cap = %d, len = %d", cap(chunk), len(chunk))
				}
			}
		})
	}
	requirePanics(t, "Chunk zero", func() { stream.Chunk(stream.Empty[int](), 0) })
	requirePanics(t, "Chunk negative", func() { stream.Chunk(stream.Empty[int](), -1) })

	chunkInput := []int{1, 2, 3, 4, 5}
	chunks := stream.Chunk(stream.FromSlice(chunkInput), 2).Collect()
	chunks[0][0] = 99
	appended := append(chunks[0], 100)
	if chunks[1][0] != 3 || chunks[2][0] != 5 {
		t.Fatalf("mutating one chunk changed another: %v", chunks)
	}
	if len(appended) != 3 || appended[2] != 100 || chunks[1][0] != 3 {
		t.Fatalf("appending one chunk changed another: appended=%v chunks=%v", appended, chunks)
	}
	if chunkInput[0] != 1 {
		t.Fatalf("mutating a chunk changed source storage: %v", chunkInput)
	}

	if got := stream.Chunk(stream.Empty[int](), math.MaxInt).Collect(); got != nil {
		t.Fatalf("empty huge Chunk = %v, want nil", got)
	}
	hugePartial := stream.Chunk(stream.Of(1, 2), math.MaxInt).Collect()
	if len(hugePartial) != 1 || !slices.Equal(hugePartial[0], []int{1, 2}) {
		t.Fatalf("short huge Chunk = %v, want [[1 2]]", hugePartial)
	}
	if cap(hugePartial[0]) != len(hugePartial[0]) {
		t.Fatalf("short huge Chunk cap = %d, len = %d", cap(hugePartial[0]), len(hugePartial[0]))
	}
}

func TestWindowAndWindowStep(t *testing.T) {
	tests := []struct {
		name       string
		input      []int
		size, step int
		want       [][]int
	}{
		{name: "short", input: []int{1, 2}, size: 3, step: 1},
		{name: "overlap", input: []int{1, 2, 3, 4, 5}, size: 3, step: 1, want: [][]int{{1, 2, 3}, {2, 3, 4}, {3, 4, 5}}},
		{name: "partial overlap", input: []int{1, 2, 3, 4, 5, 6}, size: 3, step: 2, want: [][]int{{1, 2, 3}, {3, 4, 5}}},
		{name: "equal", input: []int{1, 2, 3, 4, 5, 6, 7}, size: 3, step: 3, want: [][]int{{1, 2, 3}, {4, 5, 6}}},
		{name: "gap", input: []int{1, 2, 3, 4, 5, 6, 7, 8}, size: 2, step: 4, want: [][]int{{1, 2}, {5, 6}}},
		{name: "size one gap", input: []int{1, 2, 3, 4, 5}, size: 1, step: 2, want: [][]int{{1}, {3}, {5}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := stream.WindowStep(stream.FromSlice(test.input), test.size, test.step).Collect()
			if !slices.EqualFunc(got, test.want, slices.Equal[[]int]) {
				t.Fatalf("WindowStep = %v, want %v", got, test.want)
			}
			for _, window := range got {
				if cap(window) != len(window) {
					t.Fatalf("window cap = %d, len = %d", cap(window), len(window))
				}
			}
		})
	}

	windowInput := []int{1, 2, 3, 4, 5}
	windows := stream.Window(stream.FromSlice(windowInput), 3).Collect()
	wantWindows := [][]int{{1, 2, 3}, {2, 3, 4}, {3, 4, 5}}
	if !slices.EqualFunc(windows, wantWindows, slices.Equal[[]int]) {
		t.Fatalf("Window = %v, want %v", windows, wantWindows)
	}
	windows[0][1] = 99
	appended := append(windows[0], 100)
	if windows[1][0] != 2 || windows[2][0] != 3 || appended[3] != 100 {
		t.Fatalf("window ownership failure: windows=%v appended=%v", windows, appended)
	}
	if windowInput[1] != 2 {
		t.Fatalf("mutating a window changed source storage: %v", windowInput)
	}

	probe := new(sequenceProbe)
	first := stream.WindowStep(stream.FromSeq(instrumentedSeq([]int{1, 2, 3, 4, 5}, probe)), 2, 5).
		First().Must()
	requireSliceEqual(t, first, []int{1, 2})
	if probe.consumed.Load() != 2 {
		t.Fatalf("WindowStep pre-consumed gap: consumed = %d, want 2", probe.consumed.Load())
	}

	requirePanics(t, "Window zero", func() { stream.Window(stream.Empty[int](), 0) })
	requirePanics(t, "Window negative", func() { stream.Window(stream.Empty[int](), -1) })
	requirePanics(t, "WindowStep size zero", func() { stream.WindowStep(stream.Empty[int](), 0, 1) })
	requirePanics(t, "WindowStep step zero", func() { stream.WindowStep(stream.Empty[int](), 1, 0) })
	requirePanics(t, "WindowStep step negative", func() { stream.WindowStep(stream.Empty[int](), 1, -1) })

	if got := stream.Window(stream.Empty[int](), math.MaxInt).Collect(); got != nil {
		t.Fatalf("empty huge Window = %v, want nil", got)
	}
	if got := stream.WindowStep(stream.Of(1, 2), math.MaxInt, math.MaxInt).Collect(); got != nil {
		t.Fatalf("short huge WindowStep = %v, want nil", got)
	}
}

type sortableRecord struct {
	key   int
	label string
}

func TestSortedFuncSortedAndReverse(t *testing.T) {
	records := []sortableRecord{
		{key: 2, label: "first-two"},
		{key: 1, label: "one"},
		{key: 2, label: "second-two"},
	}
	sorted := stream.FromSlice(records).SortedFunc(func(a, b sortableRecord) int {
		return cmp.Compare(a.key, b.key)
	}).Collect()
	if sorted[0].label != "one" || sorted[1].label != "first-two" || sorted[2].label != "second-two" {
		t.Fatalf("unstable SortedFunc result: %v", sorted)
	}
	if records[0].label != "first-two" || records[1].label != "one" {
		t.Fatalf("SortedFunc modified source: %v", records)
	}
	requireSliceEqual(t, stream.Sorted(stream.Of(3, 1, 2, 1)).Collect(), []int{1, 1, 2, 3})
	requireSliceEqual(t, stream.Of(1, 2, 3).Reverse().Collect(), []int{3, 2, 1})
	if got := stream.Of(1, 2, 3).Reverse().First().Must(); got != 3 {
		t.Fatalf("Reverse First = %d", got)
	}
	if got := stream.Empty[int]().Reverse().Collect(); got != nil {
		t.Fatalf("empty Reverse = %v, want nil", got)
	}

	probe := new(sequenceProbe)
	barrier := stream.FromSeq(instrumentedSeq([]int{3, 2, 1}, probe)).SortedFunc(cmp.Compare[int])
	if probe.starts.Load() != 0 {
		t.Fatal("SortedFunc consumed at construction")
	}
	barrier.Seq()(func(value int) bool {
		if probe.consumed.Load() != 3 {
			t.Fatalf("SortedFunc emitted before exhausting source: consumed = %d", probe.consumed.Load())
		}
		return false
	})
	if probe.cleanups.Load() != 1 {
		t.Fatalf("SortedFunc source cleanup = %d", probe.cleanups.Load())
	}

	requirePanics(t, "comparator panic", func() {
		stream.Of(2, 1).SortedFunc(func(int, int) int { panic("compare") }).Collect()
	})
}

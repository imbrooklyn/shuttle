package stream_test

import (
	"iter"
	"slices"
	"testing"

	"github.com/imbrooklyn/shuttle/stream"
)

func FuzzStreamFiniteProperties(f *testing.F) {
	f.Add([]byte{}, uint8(0), uint8(1), uint8(1))
	f.Add([]byte{1, 2, 2, 3, 255}, uint8(2), uint8(3), uint8(2))
	f.Add([]byte{9, 8, 7, 6, 5}, uint8(9), uint8(2), uint8(4))
	f.Fuzz(func(t *testing.T, raw []byte, splitByte, sizeByte, stepByte uint8) {
		if len(raw) > 256 {
			t.Skip()
		}
		values := make([]int, len(raw))
		for index, value := range raw {
			values[index] = int(int8(value))
		}
		source := stream.FromSlice(values)

		requireSliceEqual(t, source.Map(func(value int) int { return value }).Collect(), values)
		requireSliceEqual(t, source.Filter(func(int) bool { return true }).Collect(), values)
		if got := source.Filter(func(int) bool { return false }).Collect(); got != nil {
			t.Fatalf("Filter(false) = %v, want nil", got)
		}

		split := int(splitByte) % (len(values) + 1)
		reconstructed := source.Take(split).Concat(source.Skip(split)).Collect()
		requireSliceEqual(t, reconstructed, values)
		requireSliceEqual(t, source.Reverse().Reverse().Collect(), values)

		size := int(sizeByte%16) + 1
		chunks := stream.Chunk(source, size).Collect()
		var flattened []int
		for _, chunk := range chunks {
			flattened = append(flattened, chunk...)
		}
		requireSliceEqual(t, flattened, values)

		step := int(stepByte%16) + 1
		gotWindows := stream.WindowStep(source, size, step).Collect()
		var wantWindows [][]int
		for start := 0; start+size <= len(values); start += step {
			window := make([]int, size)
			copy(window, values[start:start+size])
			wantWindows = append(wantWindows, window)
		}
		if !slices.EqualFunc(gotWindows, wantWindows, slices.Equal[[]int]) {
			t.Fatalf("WindowStep(%d,%d) = %v, want %v", size, step, gotWindows, wantWindows)
		}

		var wantDistinct []int
		seen := make(map[int]struct{})
		for _, value := range values {
			key := value % 7
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			wantDistinct = append(wantDistinct, value)
		}
		gotDistinct := source.DistinctBy(func(value int) int { return value % 7 }).Collect()
		requireSliceEqual(t, gotDistinct, wantDistinct)

		pairs := make([]stream.Pair[int, int], len(values))
		for index, value := range values {
			pairs[index] = stream.Pair[int, int]{First: index, Second: value}
		}
		seq2 := iter.Seq2[int, int](func(yield func(int, int) bool) {
			for _, pair := range pairs {
				if !yield(pair.First, pair.Second) {
					return
				}
			}
		})
		var roundTrip []stream.Pair[int, int]
		stream.ToSeq2(stream.FromSeq2(seq2))(func(first, second int) bool {
			roundTrip = append(roundTrip, stream.Pair[int, int]{First: first, Second: second})
			return true
		})
		if !slices.Equal(roundTrip, pairs) {
			t.Fatalf("Seq2 round trip = %v, want %v", roundTrip, pairs)
		}
	})
}

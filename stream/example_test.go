package stream_test

import (
	"cmp"
	"fmt"

	"github.com/imbrooklyn/shuttle/stream"
)

func ExampleIterate() {
	values := stream.Iterate(1, func(value int) int { return value + 1 }).
		Filter(func(value int) bool { return value%2 == 0 }).
		Map(func(value int) int { return value * value }).
		Take(5).
		Collect()

	fmt.Println(values)
	// Output: [4 16 36 64 100]
}

func ExampleFromSeq2() {
	seq2 := func(yield func(string, int) bool) {
		if !yield("a", 1) {
			return
		}
		yield("b", 2)
	}

	pairs := stream.FromSeq2(seq2).
		Map(func(pair stream.Pair[string, int]) string {
			return fmt.Sprintf("%s=%d", pair.First, pair.Second)
		}).
		Collect()

	fmt.Println(pairs)
	// Output: [a=1 b=2]
}

func ExampleZip() {
	pairs := stream.Zip(
		stream.Of("a", "b", "c"),
		stream.Of(1, 2),
	).Collect()

	fmt.Println(pairs)
	// Output: [{a 1} {b 2}]
}

func ExampleChunk() {
	chunks := stream.Chunk(stream.Range(1, 8), 3).Collect()
	fmt.Println(chunks)
	// Output: [[1 2 3] [4 5 6] [7]]
}

func ExampleChunk_retainedSlices() {
	chunks := stream.Chunk(stream.Range(1, 6), 2).Collect()
	chunks[0][0] = 99
	chunks[0] = append(chunks[0], 100)

	fmt.Println(chunks)
	// Output: [[99 2 100] [3 4] [5]]
}

func ExampleWindow() {
	windows := stream.Window(stream.Range(1, 6), 3).Collect()
	fmt.Println(windows)
	// Output: [[1 2 3] [2 3 4] [3 4 5]]
}

func ExampleWindowStep_overlap() {
	windows := stream.WindowStep(stream.Range(1, 8), 3, 2).Collect()
	fmt.Println(windows)
	// Output: [[1 2 3] [3 4 5] [5 6 7]]
}

func ExampleWindowStep_gap() {
	windows := stream.WindowStep(stream.Range(1, 10), 2, 4).Collect()
	fmt.Println(windows)
	// Output: [[1 2] [5 6]]
}

func ExampleStream_DistinctBy() {
	values := stream.Of("ant", "ape", "bear", "cat", "bird").
		DistinctBy(func(value string) byte { return value[0] }).
		Collect()

	fmt.Println(values)
	// Output: [ant bear cat]
}

func ExampleStream_SortedFunc() {
	type record struct {
		key  int
		name string
	}

	values := stream.Of(
		record{key: 2, name: "first"},
		record{key: 1, name: "middle"},
		record{key: 2, name: "second"},
	).SortedFunc(func(a, b record) int {
		return cmp.Compare(a.key, b.key)
	}).Collect()

	for _, value := range values {
		fmt.Print(value.name, " ")
	}
	// Output: middle first second
}

func ExampleStream_GroupBy() {
	groups := stream.Of("ant", "bear", "ape", "cat", "bird").
		GroupBy(func(value string) byte { return value[0] })

	for _, group := range groups {
		fmt.Printf("%c: %v\n", group.Key, group.Values)
	}
	// Output:
	// a: [ant ape]
	// b: [bear bird]
	// c: [cat]
}

package stream_test

import (
	"cmp"
	"errors"
	"slices"
	"testing"

	"github.com/imbrooklyn/shuttle/stream"
)

func TestCollectionAndCountTerminals(t *testing.T) {
	if got := stream.Empty[int]().Collect(); got != nil {
		t.Fatalf("empty Collect = %v, want nil", got)
	}
	requireSliceEqual(t, stream.Of(1, 2, 3).Collect(), []int{1, 2, 3})
	input := []int{1, 2, 3}
	collected := stream.FromSlice(input).Collect()
	collected[0] = 9
	if input[0] != 1 {
		t.Fatalf("Collect result aliases source storage: input = %v", input)
	}

	type integers []int
	var nilDestination integers
	if got := stream.Empty[int]().AppendTo(nilDestination); got != nil {
		t.Fatalf("empty AppendTo changed nilness: %v", got)
	}
	destination := integers{9}
	got := stream.Of(1, 2).AppendTo(destination)
	requireSliceEqual(t, []int(got), []int{9, 1, 2})

	if got := stream.Empty[int]().Count(); got != 0 {
		t.Fatalf("empty Count = %d", got)
	}
	if got := stream.Range(0, 1000).Count(); got != 1000 {
		t.Fatalf("Count = %d", got)
	}
}

func TestEmptyTerminalIdentitiesAndCallbackSelection(t *testing.T) {
	empty := stream.Empty[int]()
	var nilPredicate func(int) bool
	var nilForEach func(int)
	var nilForEachErr func(int) error
	var nilReduce func(int, int) int
	var nilCompare func(int, int) int
	var nilKey func(int) int
	var nilValue func(int) string
	var nilMerge func(string, string) string

	if empty.Find(nilPredicate).IsSome() || empty.At(0).IsSome() {
		t.Fatal("empty search terminal returned Some")
	}
	if empty.Any(nilPredicate) || !empty.All(nilPredicate) || !empty.None(nilPredicate) {
		t.Fatal("empty quantifier identity is incorrect")
	}
	if stream.Contains(empty, 1) {
		t.Fatal("Contains found a value in an empty Stream")
	}
	empty.ForEach(nilForEach)
	if err := empty.ForEachErr(nilForEachErr); err != nil {
		t.Fatalf("empty ForEachErr = %v", err)
	}
	if got := empty.Reduce(7, nilReduce); got != 7 {
		t.Fatalf("empty Reduce = %d", got)
	}
	if empty.ReduceFirst(nilReduce).IsSome() || empty.MinFunc(nilCompare).IsSome() || empty.MaxFunc(nilCompare).IsSome() {
		t.Fatal("empty reduction or comparator terminal returned Some")
	}
	if empty.MinBy[int](nilKey).IsSome() || empty.MaxBy[int](nilKey).IsSome() {
		t.Fatal("empty key extrema returned Some")
	}
	if stream.Min(empty).IsSome() || stream.Max(empty).IsSome() {
		t.Fatal("empty natural extrema returned Some")
	}
	if got := empty.ToMap[int, string](nilKey, nilValue); got == nil || len(got) != 0 {
		t.Fatalf("empty ToMap = %#v", got)
	}
	if got := empty.ToMapWith[int, string](nilKey, nilValue, nilMerge); got == nil || len(got) != 0 {
		t.Fatalf("empty ToMapWith = %#v", got)
	}
	if got := empty.GroupBy[int](nilKey); got != nil {
		t.Fatalf("empty GroupBy = %v", got)
	}
}

func TestPositionalAndQuantifierTerminals(t *testing.T) {
	if stream.Empty[int]().First().IsSome() || stream.Empty[int]().Last().IsSome() {
		t.Fatal("empty positional terminal returned Some")
	}
	if got := stream.Of(1, 2, 3).First().Must(); got != 1 {
		t.Fatalf("First = %d", got)
	}
	if got := stream.Of(1, 2, 3).Last().Must(); got != 3 {
		t.Fatalf("Last = %d", got)
	}

	probe := new(sequenceProbe)
	found := stream.FromSeq(instrumentedSeq([]int{1, 2, 3, 4}, probe)).Find(func(value int) bool { return value == 3 })
	if found.Must() != 3 || probe.consumed.Load() != 3 || probe.cleanups.Load() != 1 {
		t.Fatalf("Find=%v consumed=%d cleanup=%d", found, probe.consumed.Load(), probe.cleanups.Load())
	}
	if stream.Of(1, 2).Find(func(value int) bool { return value == 3 }).IsSome() {
		t.Fatal("Find returned Some without a match")
	}

	for _, test := range []struct {
		index int
		want  int
		ok    bool
	}{
		{index: -1},
		{index: 0, want: 4, ok: true},
		{index: 2, want: 6, ok: true},
		{index: 3},
	} {
		value, ok := stream.Of(4, 5, 6).At(test.index).Value()
		if ok != test.ok || value != test.want {
			t.Fatalf("At(%d) = (%d,%v), want (%d,%v)", test.index, value, ok, test.want, test.ok)
		}
	}
	started := false
	if stream.FromSeq(func(func(int) bool) { started = true }).At(-1).IsSome() || started {
		t.Fatal("At(-1) invoked upstream")
	}

	quantifierProbe := new(sequenceProbe)
	quantifierSource := stream.FromSeq(instrumentedSeq([]int{1, 2, 3}, quantifierProbe))
	if !quantifierSource.Any(func(value int) bool { return value == 2 }) || quantifierProbe.consumed.Load() != 2 {
		t.Fatalf("Any result or consumption: %d", quantifierProbe.consumed.Load())
	}
	if !stream.Of(1, 2, 3).All(func(value int) bool { return value < 4 }) {
		t.Fatal("All returned false")
	}
	if stream.Of(1, 2, 3).All(func(value int) bool { return value < 2 }) {
		t.Fatal("All failed to short-circuit rejection")
	}
	if !stream.Of(1, 2, 3).None(func(value int) bool { return value > 3 }) {
		t.Fatal("None returned false")
	}
	if stream.Of(1, 2, 3).None(func(value int) bool { return value == 2 }) {
		t.Fatal("None returned true for a match")
	}
	if stream.Empty[int]().Any(func(int) bool { return true }) ||
		!stream.Empty[int]().All(func(int) bool { return false }) ||
		!stream.Empty[int]().None(func(int) bool { return true }) {
		t.Fatal("empty quantifier identity is incorrect")
	}
	if !stream.Contains(stream.Of(1, 2, 3), 2) || stream.Contains(stream.Of(1, 2, 3), 4) {
		t.Fatal("Contains result is incorrect")
	}
	requirePanics(t, "Contains dynamic comparison", func() {
		stream.Contains(stream.Of[any]([]int{1}), any([]int{1}))
	})
}

func TestForEachAndReductions(t *testing.T) {
	var visited []int
	stream.Of(1, 2, 3).ForEach(func(value int) { visited = append(visited, value) })
	requireSliceEqual(t, visited, []int{1, 2, 3})

	sentinel := errors.New("stop")
	probe := new(sequenceProbe)
	err := stream.FromSeq(instrumentedSeq([]int{1, 2, 3}, probe)).ForEachErr(func(value int) error {
		if value == 2 {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) || err != sentinel || probe.consumed.Load() != 2 || probe.cleanups.Load() != 1 {
		t.Fatalf("ForEachErr err=%v consumed=%d cleanup=%d", err, probe.consumed.Load(), probe.cleanups.Load())
	}
	if err := stream.Empty[int]().ForEachErr(func(int) error { return sentinel }); err != nil {
		t.Fatalf("empty ForEachErr = %v", err)
	}

	reduceCalls := 0
	if got := stream.Of(1, 2, 3).Reduce(10, func(acc, value int) int {
		reduceCalls++
		return acc + value
	}); got != 16 || reduceCalls != 3 {
		t.Fatalf("Reduce = %d, calls = %d", got, reduceCalls)
	}
	if got := stream.Empty[int]().Reduce(10, func(int, int) int { panic("must not run") }); got != 10 {
		t.Fatalf("empty Reduce = %d", got)
	}
	if stream.Empty[int]().ReduceFirst(func(int, int) int { panic("must not run") }).IsSome() {
		t.Fatal("empty ReduceFirst returned Some")
	}
	reduceCalls = 0
	if got := stream.Of(2).ReduceFirst(func(int, int) int { reduceCalls++; return 0 }).Must(); got != 2 || reduceCalls != 0 {
		t.Fatalf("singleton ReduceFirst = %d, calls = %d", got, reduceCalls)
	}
	if got := stream.Of(1, 2, 3).ReduceFirst(func(acc, value int) int { return acc*10 + value }).Must(); got != 123 {
		t.Fatalf("ReduceFirst = %d", got)
	}
}

type ranked struct {
	name string
	rank int
}

func TestExtrema(t *testing.T) {
	if stream.Empty[int]().MinFunc(cmp.Compare[int]).IsSome() || stream.Empty[int]().MaxFunc(cmp.Compare[int]).IsSome() {
		t.Fatal("empty extrema returned Some")
	}
	records := stream.Of(
		ranked{name: "first-low", rank: 1},
		ranked{name: "high", rank: 3},
		ranked{name: "second-low", rank: 1},
		ranked{name: "second-high", rank: 3},
	)
	compareCalls := 0
	compareRank := func(a, b ranked) int {
		compareCalls++
		return cmp.Compare(a.rank, b.rank)
	}
	if got := records.MinFunc(compareRank).Must().name; got != "first-low" || compareCalls != 3 {
		t.Fatalf("MinFunc = %s, calls = %d", got, compareCalls)
	}
	compareCalls = 0
	if got := records.MaxFunc(compareRank).Must().name; got != "high" || compareCalls != 3 {
		t.Fatalf("MaxFunc = %s, calls = %d", got, compareCalls)
	}

	keyCalls := 0
	if got := records.MinBy(func(value ranked) int { keyCalls++; return value.rank }).Must().name; got != "first-low" || keyCalls != 4 {
		t.Fatalf("MinBy = %s, key calls = %d", got, keyCalls)
	}
	keyCalls = 0
	if got := records.MaxBy(func(value ranked) int { keyCalls++; return value.rank }).Must().name; got != "high" || keyCalls != 4 {
		t.Fatalf("MaxBy = %s, key calls = %d", got, keyCalls)
	}
	if got := stream.Min(stream.Of(3, 1, 1, 2)).Must(); got != 1 {
		t.Fatalf("Min = %d", got)
	}
	if got := stream.Max(stream.Of(3, 4, 4, 2)).Must(); got != 4 {
		t.Fatalf("Max = %d", got)
	}
}

func TestMapAndGroupTerminals(t *testing.T) {
	emptyMap := stream.Empty[int]().ToMap(func(value int) int { return value }, func(value int) int { return value })
	if emptyMap == nil || len(emptyMap) != 0 {
		t.Fatalf("empty ToMap = %#v, want non-nil empty", emptyMap)
	}

	keyCalls, valueCalls := 0, 0
	mapped := stream.Of("a", "bb", "c").ToMap(
		func(value string) int { keyCalls++; return len(value) },
		func(value string) string { valueCalls++; return value },
	)
	if keyCalls != 3 || valueCalls != 3 || mapped[1] != "c" || mapped[2] != "bb" {
		t.Fatalf("ToMap = %v, key calls = %d, value calls = %d", mapped, keyCalls, valueCalls)
	}

	mergeCalls := 0
	merged := stream.Of("a", "bb", "c", "dd").ToMapWith(
		func(value string) int { return len(value) },
		func(value string) string { return value },
		func(existing, incoming string) string {
			mergeCalls++
			return existing + incoming
		},
	)
	if mergeCalls != 2 || merged[1] != "ac" || merged[2] != "bbdd" {
		t.Fatalf("ToMapWith = %v, merge calls = %d", merged, mergeCalls)
	}
	if empty := stream.Empty[int]().ToMapWith(
		func(value int) int { return value },
		func(value int) int { return value },
		func(a, b int) int { return a + b },
	); empty == nil {
		t.Fatal("empty ToMapWith returned nil map")
	}

	groupKeyCalls := 0
	groups := stream.Of("ant", "bear", "ape", "cat", "bird").GroupBy(func(value string) byte {
		groupKeyCalls++
		return value[0]
	})
	want := []stream.Group[byte, string]{
		{Key: 'a', Values: []string{"ant", "ape"}},
		{Key: 'b', Values: []string{"bear", "bird"}},
		{Key: 'c', Values: []string{"cat"}},
	}
	if !slices.EqualFunc(groups, want, func(a, b stream.Group[byte, string]) bool {
		return a.Key == b.Key && slices.Equal(a.Values, b.Values)
	}) {
		t.Fatalf("GroupBy = %v, want %v", groups, want)
	}
	if groupKeyCalls != 5 {
		t.Fatalf("GroupBy key calls = %d, want 5", groupKeyCalls)
	}
	groups[0].Values[0] = "changed"
	if groups[1].Values[0] != "bear" {
		t.Fatalf("group Values slices alias: %v", groups)
	}
	if got := stream.Empty[int]().GroupBy(func(value int) int { return value }); got != nil {
		t.Fatalf("empty GroupBy = %v, want nil", got)
	}

	requirePanics(t, "ToMap dynamic key", func() {
		stream.Of(1).ToMap(func(int) any { return []int{1} }, func(value int) int { return value })
	})
	requirePanics(t, "ToMapWith dynamic key", func() {
		stream.Of(1).ToMapWith(func(int) any { return []int{1} }, func(value int) int { return value }, func(a, b int) int { return a + b })
	})
	requirePanics(t, "GroupBy dynamic key", func() {
		stream.Of(1).GroupBy(func(int) any { return []int{1} })
	})
}

package stream_test

import (
	"errors"
	"iter"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/imbrooklyn/shuttle/optional"
	"github.com/imbrooklyn/shuttle/stream"
)

func TestMapFilterFilterMapAndInspect(t *testing.T) {
	consumed := 0
	callbacks := 0
	source := stream.FromSeq(func(yield func(int) bool) {
		for _, value := range []int{1, 2, 3, 4} {
			consumed++
			if !yield(value) {
				return
			}
		}
	})
	pipeline := source.
		Map(func(value int) int { callbacks++; return value * 2 }).
		Filter(func(value int) bool { callbacks++; return value%4 == 0 }).
		FilterMap(func(value int) optional.Optional[string] {
			callbacks++
			if value == 4 {
				return optional.Some("four")
			}
			return optional.None[string]()
		}).
		Inspect(func(string) { callbacks++ })
	if consumed != 0 || callbacks != 0 {
		t.Fatal("pipeline construction consumed source or invoked callback")
	}
	requireSliceEqual(t, pipeline.Collect(), []string{"four"})
	if consumed != 4 || callbacks != 4+4+2+1 {
		t.Fatalf("consumed = %d callbacks = %d", consumed, callbacks)
	}

	var nilMap func(int) int
	var nilPredicate func(int) bool
	var nilFilterMap func(int) optional.Optional[int]
	var nilInspect func(int)
	if got := stream.Empty[int]().Map(nilMap).Filter(nilPredicate).FilterMap(nilFilterMap).Inspect(nilInspect).Collect(); got != nil {
		t.Fatalf("empty nil-callback pipeline = %v", got)
	}

	presentNil := stream.Of(1).FilterMap(func(int) optional.Optional[*int] {
		return optional.Some[*int](nil)
	}).Collect()
	if len(presentNil) != 1 || presentNil[0] != nil {
		t.Fatalf("FilterMap lost present nil: %v", presentNil)
	}
}

func TestGenericStreamMethodValuesAndExpressions(t *testing.T) {
	var methodValue func(func(int) string) stream.Stream[string] = stream.Of(1, 2).Map
	requireSliceEqual(t, methodValue(func(value int) string {
		return string(rune('a' + value - 1))
	}).Collect(), []string{"a", "b"})

	methodExpression := stream.Stream[int].Map[string]
	requireSliceEqual(t, methodExpression(stream.Of(3, 4), func(value int) string {
		return string(rune('a' + value - 1))
	}).Collect(), []string{"c", "d"})

	var flatMapSliceValue func(func(int) []string) stream.Stream[string] = stream.Of(1, 2).FlatMapSlice
	requireSliceEqual(t, flatMapSliceValue(func(value int) []string {
		return []string{string(rune('a' + value - 1)), "!"}
	}).Collect(), []string{"a", "!", "b", "!"})

	flatMapSliceExpression := stream.Stream[int].FlatMapSlice[string]
	requireSliceEqual(t, flatMapSliceExpression(stream.Of(3, 4), func(value int) []string {
		return []string{string(rune('a' + value - 1))}
	}).Collect(), []string{"c", "d"})
}

func TestTakeAndSkipConsumptionBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		take     int
		want     []int
		consumed int64
	}{
		{name: "zero", take: 0},
		{name: "one", take: 1, want: []int{1}, consumed: 1},
		{name: "exact", take: 3, want: []int{1, 2, 3}, consumed: 3},
		{name: "past end", take: 5, want: []int{1, 2, 3}, consumed: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			probe := new(sequenceProbe)
			got := stream.FromSeq(instrumentedSeq([]int{1, 2, 3}, probe)).Take(test.take).Collect()
			requireSliceEqual(t, got, test.want)
			if probe.consumed.Load() != test.consumed {
				t.Fatalf("consumed = %d, want %d", probe.consumed.Load(), test.consumed)
			}
			if test.take == 0 && probe.starts.Load() != 0 {
				t.Fatalf("Take(0) started source %d times", probe.starts.Load())
			}
			if test.take != 0 && probe.cleanups.Load() != 1 {
				t.Fatalf("cleanup count = %d", probe.cleanups.Load())
			}
		})
	}
	requirePanics(t, "Take negative", func() { stream.Empty[int]().Take(-1) })

	skipTests := []struct {
		n    int
		want []int
	}{
		{n: 0, want: []int{1, 2, 3}},
		{n: 1, want: []int{2, 3}},
		{n: 3},
		{n: 5},
	}
	for _, test := range skipTests {
		t.Run("Skip", func(t *testing.T) {
			requireSliceEqual(t, stream.Of(1, 2, 3).Skip(test.n).Collect(), test.want)
		})
	}
	requirePanics(t, "Skip negative", func() { stream.Empty[int]().Skip(-1) })
}

func TestTakeWhileAndSkipWhile(t *testing.T) {
	probe := new(sequenceProbe)
	predicateCalls := 0
	got := stream.FromSeq(instrumentedSeq([]int{1, 2, 3, 4}, probe)).
		TakeWhile(func(value int) bool { predicateCalls++; return value < 3 }).
		Collect()
	requireSliceEqual(t, got, []int{1, 2})
	if probe.consumed.Load() != 3 || predicateCalls != 3 || probe.cleanups.Load() != 1 {
		t.Fatalf("TakeWhile consumed=%d predicate=%d cleanup=%d", probe.consumed.Load(), predicateCalls, probe.cleanups.Load())
	}

	predicateCalls = 0
	got = stream.Of(1, 2, 3, 1).
		SkipWhile(func(value int) bool { predicateCalls++; return value < 3 }).
		Collect()
	requireSliceEqual(t, got, []int{3, 1})
	if predicateCalls != 3 {
		t.Fatalf("SkipWhile predicate calls = %d, want 3", predicateCalls)
	}

	for _, test := range []struct {
		name      string
		input     []int
		predicate func(int) bool
		wantTake  []int
		takeCalls int
		wantSkip  []int
		skipCalls int
	}{
		{name: "empty", predicate: func(int) bool { return true }},
		{name: "first false", input: []int{1, 2}, predicate: func(int) bool { return false }, wantSkip: []int{1, 2}, takeCalls: 1, skipCalls: 1},
		{name: "all true", input: []int{1, 2}, predicate: func(int) bool { return true }, wantTake: []int{1, 2}, takeCalls: 2, skipCalls: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			takeCalls := 0
			take := stream.FromSlice(test.input).TakeWhile(func(value int) bool {
				takeCalls++
				return test.predicate(value)
			}).Collect()
			requireSliceEqual(t, take, test.wantTake)
			if takeCalls != test.takeCalls {
				t.Fatalf("TakeWhile calls = %d, want %d", takeCalls, test.takeCalls)
			}

			skipCalls := 0
			skip := stream.FromSlice(test.input).SkipWhile(func(value int) bool {
				skipCalls++
				return test.predicate(value)
			}).Collect()
			requireSliceEqual(t, skip, test.wantSkip)
			if skipCalls != test.skipCalls {
				t.Fatalf("SkipWhile calls = %d, want %d", skipCalls, test.skipCalls)
			}
		})
	}
}

func TestFlatMapOrderTerminationAndCleanup(t *testing.T) {
	got := stream.Of(1, 2, 3).FlatMap(func(value int) stream.Stream[int] {
		if value == 2 {
			return stream.Empty[int]()
		}
		return stream.Of(value, -value)
	}).Collect()
	requireSliceEqual(t, got, []int{1, -1, 3, -3})

	outerConsumed := 0
	innerConsumed := 0
	got = stream.FromSeq(func(yield func(int) bool) {
		for _, value := range []int{1, 2} {
			outerConsumed++
			if !yield(value) {
				return
			}
		}
	}).FlatMap(func(int) stream.Stream[int] {
		return stream.Generate(func() int {
			innerConsumed++
			return innerConsumed
		})
	}).Take(3).Collect()
	requireSliceEqual(t, got, []int{1, 2, 3})
	if outerConsumed != 1 || innerConsumed != 3 {
		t.Fatalf("outer consumed = %d, inner consumed = %d", outerConsumed, innerConsumed)
	}

	var outerCleaned, innerCleaned atomic.Bool
	outer := stream.FromSeq(func(yield func(int) bool) {
		defer outerCleaned.Store(true)
		yield(1)
	})
	inner := func(int) stream.Stream[int] {
		return stream.FromSeq(func(yield func(int) bool) {
			defer innerCleaned.Store(true)
			for value := 0; ; value++ {
				if !yield(value) {
					return
				}
			}
		})
	}
	outer.FlatMap(inner).Take(2).Collect()
	if !outerCleaned.Load() || !innerCleaned.Load() {
		t.Fatalf("cleanup outer=%v inner=%v", outerCleaned.Load(), innerCleaned.Load())
	}
}

func TestFlatMapSliceSemantics(t *testing.T) {
	type branch struct {
		leaves []int
	}
	type root struct {
		branches []branch
	}

	roots := []root{
		{branches: []branch{{leaves: nil}, {leaves: []int{}}, {leaves: []int{1, 2}}}},
		{branches: []branch{{leaves: []int{3}}}},
	}
	rootCalls := 0
	branchCalls := 0
	pipeline := stream.FromSlice(roots).
		FlatMapSlice(func(value root) []branch {
			rootCalls++
			return value.branches
		}).
		FlatMapSlice(func(value branch) []int {
			branchCalls++
			return value.leaves
		})
	if rootCalls != 0 || branchCalls != 0 {
		t.Fatal("FlatMapSlice invoked callbacks at construction")
	}
	requireSliceEqual(t, pipeline.Collect(), []int{1, 2, 3})
	if rootCalls != 2 || branchCalls != 4 {
		t.Fatalf("first traversal calls: roots=%d branches=%d, want 2 and 4", rootCalls, branchCalls)
	}
	requireSliceEqual(t, pipeline.Collect(), []int{1, 2, 3})
	if rootCalls != 4 || branchCalls != 8 {
		t.Fatalf("second traversal calls: roots=%d branches=%d, want 4 and 8", rootCalls, branchCalls)
	}

	aliasedInner := []int{1, 2}
	var aliasedOutput []int
	stream.Of(0).FlatMapSlice(func(int) []int { return aliasedInner }).Seq()(func(value int) bool {
		aliasedOutput = append(aliasedOutput, value)
		if value == 1 {
			aliasedInner[1] = 9
		}
		return true
	})
	requireSliceEqual(t, aliasedOutput, []int{1, 9})

	probe := new(sequenceProbe)
	callbackCalls := 0
	emitted := 0
	got := stream.FromSeq(instrumentedSeq([][]int{{1, 2, 3}, {4}}, probe)).
		FlatMapSlice(func(values []int) []int {
			callbackCalls++
			return values
		}).
		Inspect(func(int) { emitted++ }).
		Take(2).
		Collect()
	requireSliceEqual(t, got, []int{1, 2})
	if probe.consumed.Load() != 1 || probe.cleanups.Load() != 1 || callbackCalls != 1 || emitted != 2 {
		t.Fatalf("early stop consumed=%d cleanup=%d callbacks=%d emitted=%d",
			probe.consumed.Load(), probe.cleanups.Load(), callbackCalls, emitted)
	}

	var nilCallback func(int) []string
	if got := stream.Empty[int]().FlatMapSlice(nilCallback).Collect(); got != nil {
		t.Fatalf("empty FlatMapSlice with nil callback = %v, want nil", got)
	}
	requirePanics(t, "reached nil FlatMapSlice callback", func() {
		stream.Of(1).FlatMapSlice(nilCallback).Collect()
	})

	callbackPanic := errors.New("flat-map-slice callback")
	requirePanicValue(t, callbackPanic, func() {
		stream.Of(1).FlatMapSlice(func(int) []int { panic(callbackPanic) }).Collect()
	})
	downstreamPanic := errors.New("flat-map-slice downstream")
	requirePanicValue(t, downstreamPanic, func() {
		stream.Of(1).FlatMapSlice(func(int) []int { return []int{1, 2} }).Seq()(func(int) bool {
			panic(downstreamPanic)
		})
	})

	position := 0
	singleUse := stream.FromSeq(func(yield func([]int) bool) {
		values := [][]int{{1}, {2}}
		for position < len(values) {
			value := values[position]
			position++
			if !yield(value) {
				return
			}
		}
	}).FlatMapSlice(func(values []int) []int { return values })
	requireSliceEqual(t, singleUse.Collect(), []int{1, 2})
	if got := singleUse.Collect(); got != nil {
		t.Fatalf("replayed single-use FlatMapSlice = %v, want nil", got)
	}
}

func TestScanSemanticsAndAliasing(t *testing.T) {
	if got := stream.Empty[int]().Scan(10, func(acc, value int) int { return acc + value }).Collect(); got != nil {
		t.Fatalf("empty Scan = %v, want nil", got)
	}
	requireSliceEqual(t, stream.Of(1, 2, 3).Scan(10, func(acc, value int) int {
		return acc + value
	}).Collect(), []int{11, 13, 16})

	initial := []int{0}
	aliased := stream.Of(1, 2).Scan(initial, func(acc []int, value int) []int {
		acc[0] += value
		return acc
	})
	first := aliased.Collect()
	if first[0][0] != 3 || first[1][0] != 3 {
		t.Fatalf("emitted shallow accumulators did not alias: %v", first)
	}
	second := aliased.Collect()
	if second[0][0] != 6 || initial[0] != 6 {
		t.Fatalf("initial reference storage was not shared across traversals: second=%v initial=%v", second, initial)
	}
}

func TestEnumerateAndConcat(t *testing.T) {
	pairs := stream.Enumerate(stream.Of("a", "b")).Collect()
	wantPairs := []stream.Pair[int, string]{{First: 0, Second: "a"}, {First: 1, Second: "b"}}
	requireSliceEqual(t, pairs, wantPairs)
	if got := stream.Enumerate(stream.Of("first", "second")).First().Must(); got != (stream.Pair[int, string]{First: 0, Second: "first"}) {
		t.Fatalf("Enumerate First = %v", got)
	}

	others := []stream.Stream[int]{stream.Of(2), stream.Of(3)}
	concatenated := stream.Of(1).Concat(others...)
	others[0] = stream.Of(9)
	requireSliceEqual(t, concatenated.Collect(), []int{1, 2, 3})

	started := false
	later := stream.FromSeq(func(yield func(int) bool) {
		started = true
		yield(9)
	})
	if got := stream.Of(1, 2).Concat(later).First().Must(); got != 1 {
		t.Fatalf("Concat First = %d", got)
	}
	if started {
		t.Fatal("Concat started later source after downstream termination")
	}
	if got := stream.Empty[int]().Concat(stream.Of(4, 5)).First().Must(); got != 4 {
		t.Fatalf("Concat later-source First = %d", got)
	}
}

func TestPanicPropagationAndSourceCleanup(t *testing.T) {
	cleaned := false
	source := stream.FromSeq(func(yield func(int) bool) {
		defer func() { cleaned = true }()
		yield(1)
	})
	requirePanics(t, "Map callback", func() {
		source.Map(func(int) int { panic("callback panic") }).Collect()
	})
	if !cleaned {
		t.Fatal("source defer did not execute after callback panic")
	}

	sentinel := errors.New("sentinel")
	requirePanics(t, "source panic", func() {
		stream.FromSeq[int](func(func(int) bool) { panic(sentinel) }).Collect()
	})
}

func TestConcurrentTraversalOfBuiltInImmutableSource(t *testing.T) {
	source := stream.FromSlice([]int{1, 2, 3})
	const traversals = 16
	var wait sync.WaitGroup
	wait.Add(traversals)
	for range traversals {
		go func() {
			defer wait.Done()
			requireSliceEqual(t, source.Collect(), []int{1, 2, 3})
		}()
	}
	wait.Wait()
}

func TestConcurrentTraversalUsesIndependentOperatorState(t *testing.T) {
	source := stream.Distinct(stream.FromSlice([]int{1, 2, 2, 3, 1}))
	const traversals = 16
	var wait sync.WaitGroup
	wait.Add(traversals)
	for range traversals {
		go func() {
			defer wait.Done()
			requireSliceEqual(t, source.Collect(), []int{1, 2, 3})
		}()
	}
	wait.Wait()
}

func TestAdaptersPropagateDownstreamFalse(t *testing.T) {
	operators := map[string]stream.Stream[int]{
		"Map":       stream.Range(0, 10).Map(func(value int) int { return value }),
		"Filter":    stream.Range(0, 10).Filter(func(int) bool { return true }),
		"Inspect":   stream.Range(0, 10).Inspect(func(int) {}),
		"Skip":      stream.Range(0, 10).Skip(1),
		"SkipWhile": stream.Range(0, 10).SkipWhile(func(int) bool { return false }),
	}
	for name, operator := range operators {
		t.Run(name, func(t *testing.T) {
			calls := 0
			operator.Seq()(func(int) bool {
				calls++
				return false
			})
			if calls != 1 {
				t.Fatalf("downstream calls = %d, want 1", calls)
			}
		})
	}
}

var _ iter.Seq[int] = stream.Stream[int]{}.Seq()

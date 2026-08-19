package comparator_test

import (
	"cmp"
	"math"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/imbrooklyn/shuttle/comparator"
	"github.com/imbrooklyn/shuttle/stream"
)

type operand struct {
	label string
	key   int
}

type record struct {
	rank      int
	name      string
	id        int
	createdAt time.Time
}

type namedOrdered int

func TestFuncZeroAndNil(t *testing.T) {
	var zero comparator.Func[int]
	if zero != nil {
		t.Fatal("zero Func is not nil")
	}

	reversed := zero.Reverse()
	combined := zero.Then(func(int, int) int { return 0 })
	thenBy := zero.ThenBy(func(value int) int { return value })
	thenByDescending := zero.ThenByDescending(func(value int) int { return value })
	thenOn := zero.ThenOn(func(value int) int { return value }, comparator.Ordered[int]())
	thenOnDescending := zero.ThenOnDescending(func(value int) int { return value }, comparator.Ordered[int]())
	requirePanic(t, "zero Reverse evaluation", func() { reversed(1, 2) })
	requirePanic(t, "zero Then evaluation", func() { combined(1, 2) })
	requirePanic(t, "zero ThenBy evaluation", func() { thenBy(1, 2) })
	requirePanic(t, "zero ThenByDescending evaluation", func() { thenByDescending(1, 2) })
	requirePanic(t, "zero ThenOn evaluation", func() { thenOn(1, 2) })
	requirePanic(t, "zero ThenOnDescending evaluation", func() { thenOnDescending(1, 2) })

	var explicitNil comparator.Func[int] = nil
	requirePanic(t, "explicit nil evaluation", func() { explicitNil(1, 2) })
}

func TestOrdered(t *testing.T) {
	ints := comparator.Ordered[int]()
	for _, test := range []struct {
		left  int
		right int
		want  int
	}{
		{left: -1, right: 2, want: -1},
		{left: 3, right: 3, want: 0},
		{left: 4, right: 1, want: 1},
	} {
		if got := ints(test.left, test.right); got != test.want {
			t.Fatalf("Ordered(%d, %d) = %d, want %d", test.left, test.right, got, test.want)
		}
	}

	if got := comparator.Ordered[string]()("alpha", "beta"); got >= 0 {
		t.Fatalf("Ordered string result = %d, want negative", got)
	}
	if got := comparator.Ordered[namedOrdered]()(2, 1); got <= 0 {
		t.Fatalf("Ordered named result = %d, want positive", got)
	}

	floats := comparator.Ordered[float64]()
	for _, test := range []struct {
		name        string
		left, right float64
	}{
		{name: "NaN before number", left: math.NaN(), right: 1},
		{name: "number after NaN", left: 1, right: math.NaN()},
		{name: "NaNs equivalent", left: math.NaN(), right: math.NaN()},
		{name: "signed zeros equivalent", left: math.Copysign(0, -1), right: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, want := floats(test.left, test.right), cmp.Compare(test.left, test.right); got != want {
				t.Fatalf("Ordered float result = %d, want %d", got, want)
			}
		})
	}
}

func TestByProjectionOrderAndCount(t *testing.T) {
	var order []string
	compare := comparator.By(func(value operand) int {
		order = append(order, value.label)
		return value.key
	})
	if len(order) != 0 {
		t.Fatal("By evaluated its projection during construction")
	}
	if got := compare(operand{"left", 1}, operand{"right", 2}); got >= 0 {
		t.Fatalf("By result = %d, want negative", got)
	}
	if want := []string{"left", "right"}; !slices.Equal(order, want) {
		t.Fatalf("By projection order = %v, want %v", order, want)
	}

	leftPanic := &panicMarker{"left"}
	order = nil
	panicking := comparator.By(func(value operand) int {
		order = append(order, value.label)
		if value.label == "left" {
			panic(leftPanic)
		}
		return value.key
	})
	requirePanicValue(t, leftPanic, func() {
		panicking(operand{"left", 1}, operand{"right", 2})
	})
	if want := []string{"left"}; !slices.Equal(order, want) {
		t.Fatalf("By calls after left panic = %v, want %v", order, want)
	}

	rightPanic := &panicMarker{"right"}
	order = nil
	panicking = comparator.By(func(value operand) int {
		order = append(order, value.label)
		if value.label == "right" {
			panic(rightPanic)
		}
		return value.key
	})
	requirePanicValue(t, rightPanic, func() {
		panicking(operand{"left", 1}, operand{"right", 2})
	})
	if want := []string{"left", "right"}; !slices.Equal(order, want) {
		t.Fatalf("By calls before right panic = %v, want %v", order, want)
	}

	var nilKey func(operand) int
	nilComparator := comparator.By(nilKey)
	requirePanic(t, "nil By key", func() {
		nilComparator(operand{}, operand{})
	})
}

func TestByDescendingDirectionAndProjectionOrder(t *testing.T) {
	var order []string
	compare := comparator.ByDescending(func(value operand) int {
		order = append(order, value.label)
		return value.key
	})
	if got := compare(operand{"left", 1}, operand{"right", 2}); got <= 0 {
		t.Fatalf("ByDescending result = %d, want positive", got)
	}
	if want := []string{"left", "right"}; !slices.Equal(order, want) {
		t.Fatalf("ByDescending projection order = %v, want %v", order, want)
	}
}

func TestOnOrderCountAndResult(t *testing.T) {
	var order []string
	compareCalls := 0
	compare := comparator.On(
		func(value operand) int {
			order = append(order, "project "+value.label)
			return value.key
		},
		func(left, right int) int {
			compareCalls++
			order = append(order, "compare")
			if left != 3 || right != 5 {
				t.Fatalf("On comparator arguments = (%d, %d), want (3, 5)", left, right)
			}
			return -17
		},
	)
	if len(order) != 0 || compareCalls != 0 {
		t.Fatal("On evaluated callbacks during construction")
	}
	if got := compare(operand{"left", 3}, operand{"right", 5}); got != -17 {
		t.Fatalf("On result = %d, want -17", got)
	}
	if want := []string{"project left", "project right", "compare"}; !slices.Equal(order, want) {
		t.Fatalf("On callback order = %v, want %v", order, want)
	}
	if compareCalls != 1 {
		t.Fatalf("On comparator calls = %d, want 1", compareCalls)
	}
}

func TestOnDescendingOrderCountAndSign(t *testing.T) {
	for _, test := range []struct {
		name   string
		result int
		want   int
	}{
		{name: "minimum int", result: math.MinInt, want: 1},
		{name: "negative", result: -17, want: 1},
		{name: "zero", result: 0, want: 0},
		{name: "positive", result: 23, want: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var order []string
			compareCalls := 0
			compare := comparator.OnDescending(
				func(value operand) int {
					order = append(order, "project "+value.label)
					return value.key
				},
				func(left, right int) int {
					compareCalls++
					order = append(order, "compare")
					if left != 3 || right != 5 {
						t.Fatalf("OnDescending comparator arguments = (%d, %d), want (3, 5)", left, right)
					}
					return test.result
				},
			)
			if len(order) != 0 || compareCalls != 0 {
				t.Fatal("OnDescending evaluated callbacks during construction")
			}
			if got := compare(operand{"left", 3}, operand{"right", 5}); got != test.want {
				t.Fatalf("OnDescending result = %d, want %d", got, test.want)
			}
			if want := []string{"project left", "project right", "compare"}; !slices.Equal(order, want) {
				t.Fatalf("OnDescending callback order = %v, want %v", order, want)
			}
			if compareCalls != 1 {
				t.Fatalf("OnDescending comparator calls = %d, want 1", compareCalls)
			}
		})
	}
}

func TestOnPanicPaths(t *testing.T) {
	tests := []struct {
		name       string
		panicLabel string
		wantOrder  []string
	}{
		{name: "left projection", panicLabel: "left", wantOrder: []string{"left"}},
		{name: "right projection", panicLabel: "right", wantOrder: []string{"left", "right"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			marker := &panicMarker{test.panicLabel}
			var order []string
			compare := comparator.On(
				func(value operand) int {
					order = append(order, value.label)
					if value.label == test.panicLabel {
						panic(marker)
					}
					return value.key
				},
				func(int, int) int {
					order = append(order, "compare")
					return 0
				},
			)
			requirePanicValue(t, marker, func() {
				compare(operand{"left", 1}, operand{"right", 2})
			})
			if !slices.Equal(order, test.wantOrder) {
				t.Fatalf("On panic order = %v, want %v", order, test.wantOrder)
			}
		})
	}

	comparePanic := &panicMarker{"compare"}
	var order []string
	compare := comparator.On(
		func(value operand) int {
			order = append(order, value.label)
			return value.key
		},
		func(int, int) int {
			order = append(order, "compare")
			panic(comparePanic)
		},
	)
	requirePanicValue(t, comparePanic, func() {
		compare(operand{"left", 1}, operand{"right", 2})
	})
	if want := []string{"left", "right", "compare"}; !slices.Equal(order, want) {
		t.Fatalf("On comparator panic order = %v, want %v", order, want)
	}

	var nilProject func(operand) int
	requirePanic(t, "nil On projection", func() {
		comparator.On(nilProject, comparator.Ordered[int]())(operand{}, operand{})
	})

	projectCalls := 0
	var nilCompare comparator.Func[int]
	nilOn := comparator.On(func(operand) int {
		projectCalls++
		return 0
	}, nilCompare)
	requirePanic(t, "nil On comparator", func() {
		nilOn(operand{}, operand{})
	})
	if projectCalls != 2 {
		t.Fatalf("On projections before nil comparator = %d, want 2", projectCalls)
	}
}

func TestOnDescendingPanicAndNilPaths(t *testing.T) {
	for _, panicLabel := range []string{"left", "right"} {
		t.Run(panicLabel+" projection", func(t *testing.T) {
			marker := &panicMarker{panicLabel}
			var order []string
			compare := comparator.OnDescending(
				func(value operand) int {
					order = append(order, value.label)
					if value.label == panicLabel {
						panic(marker)
					}
					return value.key
				},
				func(int, int) int {
					order = append(order, "compare")
					return 0
				},
			)
			requirePanicValue(t, marker, func() {
				compare(operand{"left", 1}, operand{"right", 2})
			})
			want := []string{"left"}
			if panicLabel == "right" {
				want = append(want, "right")
			}
			if !slices.Equal(order, want) {
				t.Fatalf("OnDescending projection panic order = %v, want %v", order, want)
			}
		})
	}

	marker := &panicMarker{"OnDescending compare"}
	var order []string
	panicking := comparator.OnDescending(
		func(value operand) int {
			order = append(order, value.label)
			return value.key
		},
		func(int, int) int {
			order = append(order, "compare")
			panic(marker)
		},
	)
	requirePanicValue(t, marker, func() {
		panicking(operand{"left", 1}, operand{"right", 2})
	})
	if want := []string{"left", "right", "compare"}; !slices.Equal(order, want) {
		t.Fatalf("OnDescending panic order = %v, want %v", order, want)
	}

	var nilProject func(operand) int
	requirePanic(t, "nil OnDescending projection", func() {
		comparator.OnDescending(nilProject, comparator.Ordered[int]())(operand{}, operand{})
	})

	projectCalls := 0
	var nilCompare comparator.Func[int]
	nilOn := comparator.OnDescending(func(operand) int {
		projectCalls++
		return 0
	}, nilCompare)
	requirePanic(t, "nil OnDescending comparator", func() {
		nilOn(operand{}, operand{})
	})
	if projectCalls != 2 {
		t.Fatalf("OnDescending projections before nil comparator = %d, want 2", projectCalls)
	}
}

func TestProjectedComparatorsDoNotCache(t *testing.T) {
	byCalls := 0
	by := comparator.By(func(value operand) int {
		byCalls++
		return value.key
	})
	for range 2 {
		by(operand{key: 1}, operand{key: 2})
	}
	if byCalls != 4 {
		t.Fatalf("By projection calls across evaluations = %d, want 4", byCalls)
	}

	projectCalls := 0
	compareCalls := 0
	on := comparator.On(
		func(value operand) int {
			projectCalls++
			return value.key
		},
		func(left, right int) int {
			compareCalls++
			return cmp.Compare(left, right)
		},
	)
	for range 2 {
		on(operand{key: 1}, operand{key: 2})
	}
	if projectCalls != 4 || compareCalls != 2 {
		t.Fatalf("On calls across evaluations = projections %d, comparisons %d; want 4, 2", projectCalls, compareCalls)
	}

	projectCalls = 0
	compareCalls = 0
	onDescending := comparator.OnDescending(
		func(value operand) int {
			projectCalls++
			return value.key
		},
		func(left, right int) int {
			compareCalls++
			return cmp.Compare(left, right)
		},
	)
	for range 2 {
		onDescending(operand{key: 1}, operand{key: 2})
	}
	if projectCalls != 4 || compareCalls != 2 {
		t.Fatalf("OnDescending calls across evaluations = projections %d, comparisons %d; want 4, 2", projectCalls, compareCalls)
	}

	keyCalls := 0
	fluent := comparator.Func[operand](func(operand, operand) int { return 0 }).
		ThenBy(func(value operand) int {
			keyCalls++
			return value.key
		})
	for range 2 {
		fluent(operand{key: 1}, operand{key: 2})
	}
	if keyCalls != 4 {
		t.Fatalf("ThenBy key calls across evaluations = %d, want 4", keyCalls)
	}
}

func TestReverseSignCountOrderAndPanic(t *testing.T) {
	for _, test := range []struct {
		name   string
		result int
		want   int
	}{
		{name: "minimum int", result: math.MinInt, want: 1},
		{name: "negative", result: -7, want: 1},
		{name: "zero", result: 0, want: 0},
		{name: "positive", result: 9, want: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			base := comparator.Func[int](func(left, right int) int {
				calls++
				if left != 4 || right != 8 {
					t.Fatalf("Reverse receiver arguments = (%d, %d), want (4, 8)", left, right)
				}
				return test.result
			})
			if got := base.Reverse()(4, 8); got != test.want {
				t.Fatalf("Reverse result = %d, want %d", got, test.want)
			}
			if calls != 1 {
				t.Fatalf("Reverse receiver calls = %d, want 1", calls)
			}
		})
	}

	base := comparator.Func[int](func(left, right int) int { return left - right })
	for _, pair := range [][2]int{{1, 2}, {2, 2}, {3, 2}} {
		if got, want := sign(base.Reverse().Reverse()(pair[0], pair[1])), sign(base(pair[0], pair[1])); got != want {
			t.Fatalf("double Reverse sign = %d, want %d", got, want)
		}
	}

	marker := &panicMarker{"reverse"}
	panicking := comparator.Func[int](func(int, int) int { panic(marker) }).Reverse()
	requirePanicValue(t, marker, func() { panicking(1, 2) })

	var nilBase comparator.Func[int]
	nilReverse := nilBase.Reverse()
	requirePanic(t, "nil Reverse receiver", func() { nilReverse(1, 2) })
}

func TestThenOrderShortCircuitAndRepeatedEvaluation(t *testing.T) {
	var order []string
	calls := map[string]int{}
	level := func(name string, result int) comparator.Func[int] {
		return func(left, right int) int {
			if left != 1 || right != 2 {
				t.Fatalf("Then arguments = (%d, %d), want (1, 2)", left, right)
			}
			order = append(order, name)
			calls[name]++
			return result
		}
	}

	combined := level("primary", 0).Then(level("second", 7), level("third", -1))
	if len(order) != 0 {
		t.Fatal("Then evaluated comparators during construction")
	}
	for evaluation := 0; evaluation < 2; evaluation++ {
		if got := combined(1, 2); got != 7 {
			t.Fatalf("Then result = %d, want 7", got)
		}
	}
	if want := []string{"primary", "second", "primary", "second"}; !slices.Equal(order, want) {
		t.Fatalf("Then order = %v, want %v", order, want)
	}
	if calls["primary"] != 2 || calls["second"] != 2 || calls["third"] != 0 {
		t.Fatalf("Then calls = %v, want primary=2 second=2 third=0", calls)
	}

	if got := level("only", -9).Then()(1, 2); got != -9 {
		t.Fatalf("Then with no others = %d, want -9", got)
	}
	if got := level("zero", 0).Then(level("also zero", 0))(1, 2); got != 0 {
		t.Fatalf("all-zero Then = %d, want 0", got)
	}
}

func TestThenReachedAndSkippedNil(t *testing.T) {
	var nilCompare comparator.Func[int]
	skipped := comparator.Func[int](func(int, int) int { return 1 }).Then(nilCompare)
	if got := skipped(1, 2); got != 1 {
		t.Fatalf("Then skipped nil result = %d, want 1", got)
	}

	reached := comparator.Func[int](func(int, int) int { return 0 }).Then(nilCompare)
	requirePanic(t, "reached nil comparator", func() { reached(1, 2) })

	skippedLater := comparator.Func[int](func(int, int) int { return 0 }).Then(
		func(int, int) int { return -1 },
		nilCompare,
	)
	if got := skippedLater(1, 2); got != -1 {
		t.Fatalf("Then skipped later nil result = %d, want -1", got)
	}
}

func TestThenSnapshotAliasingAndPanic(t *testing.T) {
	others := []comparator.Func[int]{
		func(int, int) int { return 0 },
		func(int, int) int { return 1 },
	}
	combined := comparator.Func[int](func(int, int) int { return 0 }).Then(others...)
	others[0] = func(int, int) int { return -1 }
	if got := combined(1, 2); got != 1 {
		t.Fatalf("Then did not snapshot descriptors: got %d, want 1", got)
	}

	direction := 1
	captured := comparator.Func[int](func(int, int) int { return direction })
	aliased := comparator.Func[int](func(int, int) int { return 0 }).Then(captured)
	direction = -1
	if got := aliased(1, 2); got != -1 {
		t.Fatalf("Then lost captured-state aliasing: got %d, want -1", got)
	}

	marker := &panicMarker{"then"}
	panicking := comparator.Func[int](func(int, int) int { return 0 }).Then(
		func(int, int) int { panic(marker) },
	)
	requirePanicValue(t, marker, func() { panicking(1, 2) })
}

func TestThenByLevelsOrderShortCircuitAndDirection(t *testing.T) {
	var order []string
	primary := comparator.Func[operand](func(left, right operand) int {
		order = append(order, "primary")
		return 0
	})
	ascending := primary.ThenBy(func(value operand) int {
		order = append(order, "key "+value.label)
		return value.key
	})
	if len(order) != 0 {
		t.Fatal("ThenBy evaluated callbacks during construction")
	}
	if got := ascending(operand{"left", 1}, operand{"right", 2}); got >= 0 {
		t.Fatalf("ThenBy result = %d, want negative", got)
	}
	if want := []string{"primary", "key left", "key right"}; !slices.Equal(order, want) {
		t.Fatalf("ThenBy callback order = %v, want %v", order, want)
	}

	order = nil
	descending := primary.ThenByDescending(func(value operand) int {
		order = append(order, "key "+value.label)
		return value.key
	})
	if got := descending(operand{"left", 1}, operand{"right", 2}); got <= 0 {
		t.Fatalf("ThenByDescending result = %d, want positive", got)
	}
	if want := []string{"primary", "key left", "key right"}; !slices.Equal(order, want) {
		t.Fatalf("ThenByDescending callback order = %v, want %v", order, want)
	}

	var nilKey func(operand) int
	skipped := comparator.Func[operand](func(operand, operand) int { return 9 }).ThenBy(nilKey)
	if got := skipped(operand{}, operand{}); got != 9 {
		t.Fatalf("ThenBy skipped nil key result = %d, want 9", got)
	}
	reached := comparator.Func[operand](func(operand, operand) int { return 0 }).ThenByDescending(nilKey)
	requirePanic(t, "reached nil ThenByDescending key", func() {
		reached(operand{}, operand{})
	})

	marker := &panicMarker{"ThenBy key"}
	panicking := comparator.Func[operand](func(operand, operand) int { return 0 }).ThenBy(func(value operand) int {
		if value.label == "right" {
			panic(marker)
		}
		return value.key
	})
	requirePanicValue(t, marker, func() {
		panicking(operand{"left", 1}, operand{"right", 2})
	})
}

func TestThenOnLevelsOrderShortCircuitDirectionAndNil(t *testing.T) {
	var order []string
	primary := comparator.Func[operand](func(left, right operand) int {
		order = append(order, "primary")
		return 0
	})
	project := func(value operand) int {
		order = append(order, "project "+value.label)
		return value.key
	}
	custom := comparator.Func[int](func(left, right int) int {
		order = append(order, "compare")
		if left != 3 || right != 5 {
			t.Fatalf("ThenOn comparator arguments = (%d, %d), want (3, 5)", left, right)
		}
		return -17
	})

	ascending := primary.ThenOn(project, custom)
	if len(order) != 0 {
		t.Fatal("ThenOn evaluated callbacks during construction")
	}
	if got := ascending(operand{"left", 3}, operand{"right", 5}); got != -17 {
		t.Fatalf("ThenOn result = %d, want -17", got)
	}
	if want := []string{"primary", "project left", "project right", "compare"}; !slices.Equal(order, want) {
		t.Fatalf("ThenOn callback order = %v, want %v", order, want)
	}

	order = nil
	descendingCompare := comparator.Func[int](func(left, right int) int {
		order = append(order, "compare")
		if left != 3 || right != 5 {
			t.Fatalf("ThenOnDescending comparator arguments = (%d, %d), want (3, 5)", left, right)
		}
		return math.MinInt
	})
	descending := primary.ThenOnDescending(project, descendingCompare)
	if got := descending(operand{"left", 3}, operand{"right", 5}); got != 1 {
		t.Fatalf("ThenOnDescending result = %d, want 1", got)
	}
	if want := []string{"primary", "project left", "project right", "compare"}; !slices.Equal(order, want) {
		t.Fatalf("ThenOnDescending callback order = %v, want %v", order, want)
	}

	var nilProject func(operand) int
	var nilCompare comparator.Func[int]
	skippedProject := comparator.Func[operand](func(operand, operand) int { return -3 }).ThenOn(
		nilProject,
		nilCompare,
	)
	if got := skippedProject(operand{}, operand{}); got != -3 {
		t.Fatalf("ThenOn skipped nil callbacks result = %d, want -3", got)
	}
	requirePanic(t, "reached nil ThenOn projection", func() {
		comparator.Func[operand](func(operand, operand) int { return 0 }).ThenOn(
			nilProject,
			nilCompare,
		)(operand{}, operand{})
	})

	projectCalls := 0
	reachedCompare := comparator.Func[operand](func(operand, operand) int { return 0 }).ThenOnDescending(
		func(operand) int {
			projectCalls++
			return 0
		},
		nilCompare,
	)
	requirePanic(t, "reached nil ThenOnDescending comparator", func() {
		reachedCompare(operand{}, operand{})
	})
	if projectCalls != 2 {
		t.Fatalf("ThenOnDescending projections before nil comparator = %d, want 2", projectCalls)
	}

	marker := &panicMarker{"ThenOn comparator"}
	panicking := comparator.Func[operand](func(operand, operand) int { return 0 }).ThenOn(
		func(value operand) int { return value.key },
		func(int, int) int { panic(marker) },
	)
	requirePanicValue(t, marker, func() {
		panicking(operand{}, operand{})
	})

	projectionMarker := &panicMarker{"ThenOn right projection"}
	compareCalls := 0
	projectionPanic := comparator.Func[operand](func(operand, operand) int { return 0 }).ThenOnDescending(
		func(value operand) int {
			if value.label == "right" {
				panic(projectionMarker)
			}
			return value.key
		},
		func(int, int) int {
			compareCalls++
			return 0
		},
	)
	requirePanicValue(t, projectionMarker, func() {
		projectionPanic(operand{"left", 1}, operand{"right", 2})
	})
	if compareCalls != 0 {
		t.Fatalf("ThenOnDescending comparator calls after projection panic = %d, want 0", compareCalls)
	}
}

func TestWholeReverseDiffersFromOneDescendingLevel(t *testing.T) {
	ascending := comparator.By(func(value record) int { return value.rank }).
		ThenBy(func(value record) string { return value.name })
	wholeReverse := ascending.Reverse()
	mixedDirection := comparator.By(func(value record) int { return value.rank }).
		ThenByDescending(func(value record) string { return value.name })

	left := record{rank: 1, name: "z"}
	right := record{rank: 2, name: "a"}
	if got := wholeReverse(left, right); got <= 0 {
		t.Fatalf("whole Reverse primary result = %d, want positive", got)
	}
	if got := mixedDirection(left, right); got >= 0 {
		t.Fatalf("one descending level primary result = %d, want negative", got)
	}

	left.rank = right.rank
	if got := wholeReverse(left, right); got >= 0 {
		t.Fatalf("whole Reverse tie result = %d, want negative", got)
	}
	if got := mixedDirection(left, right); got >= 0 {
		t.Fatalf("one descending level tie result = %d, want negative", got)
	}
}

func TestStandardLibraryAndStreamInteroperability(t *testing.T) {
	compare := comparator.By(func(value record) int { return value.rank }).
		ThenByDescending(func(value record) string { return value.name })
	input := []record{
		{rank: 2, name: "beta", id: 1},
		{rank: 1, name: "alpha", id: 2},
		{rank: 1, name: "gamma", id: 3},
		{rank: 1, name: "gamma", id: 4},
	}
	want := []record{input[2], input[3], input[1], input[0]}

	stable := slices.Clone(input)
	slices.SortStableFunc(stable, compare)
	if !slices.Equal(stable, want) {
		t.Fatalf("slices.SortStableFunc = %v, want %v", stable, want)
	}

	unstable := slices.Clone(input)
	slices.SortFunc(unstable, compare)
	if !slices.IsSortedFunc(unstable, compare) {
		t.Fatalf("slices.SortFunc result is not sorted: %v", unstable)
	}

	if got := slices.MinFunc(input, compare); got != input[2] {
		t.Fatalf("slices.MinFunc = %v, want %v", got, input[2])
	}
	if got := slices.MaxFunc(input, compare); got != input[0] {
		t.Fatalf("slices.MaxFunc = %v, want %v", got, input[0])
	}

	if got := stream.FromSlice(input).SortedFunc(compare).Collect(); !slices.Equal(got, want) {
		t.Fatalf("Stream.SortedFunc = %v, want %v", got, want)
	}
	if got := stream.FromSlice(input).MinFunc(compare).Must(); got != input[2] {
		t.Fatalf("Stream.MinFunc = %v, want %v", got, input[2])
	}
	if got := stream.FromSlice(input).MaxFunc(compare).Must(); got != input[0] {
		t.Fatalf("Stream.MaxFunc = %v, want %v", got, input[0])
	}
}

func TestInferenceMethodValuesExpressionsAndAssignability(t *testing.T) {
	inferred := comparator.By(func(value record) namedOrdered { return namedOrdered(value.rank) })
	explicit := comparator.By[record, namedOrdered](func(value record) namedOrdered {
		return namedOrdered(value.rank)
	})
	if inferred(record{rank: 1}, record{rank: 2}) != explicit(record{rank: 1}, record{rank: 2}) {
		t.Fatal("By inference and explicit instantiation disagree")
	}

	var reverseInferred comparator.Func[record] = comparator.On(
		func(value record) string { return value.name },
		cmp.Compare,
	)
	if got := reverseInferred(record{name: "a"}, record{name: "b"}); got >= 0 {
		t.Fatalf("On reverse inference result = %d, want negative", got)
	}

	base := comparator.Ordered[int]()
	var thenValue func(...comparator.Func[int]) comparator.Func[int] = base.Then
	if got := thenValue(func(int, int) int { return 1 })(1, 2); got >= 0 {
		t.Fatalf("Then method value result = %d, want receiver result", got)
	}
	thenExpression := comparator.Func[int].Then
	if got := thenExpression(func(int, int) int { return 0 }, func(int, int) int { return 1 })(1, 2); got != 1 {
		t.Fatalf("Then method expression result = %d, want 1", got)
	}

	var reverseValue func() comparator.Func[int] = base.Reverse
	if got := reverseValue()(1, 2); got <= 0 {
		t.Fatalf("Reverse method value result = %d, want positive", got)
	}
	reverseExpression := comparator.Func[int].Reverse
	if got := reverseExpression(base)(1, 2); got <= 0 {
		t.Fatalf("Reverse method expression result = %d, want positive", got)
	}

	var unnamed func(int, int) int = base
	var named comparator.Func[int] = unnamed
	if got := named(1, 2); got >= 0 {
		t.Fatalf("named/unnamed assignability result = %d, want negative", got)
	}

	byCreatedAt := comparator.On(
		func(value record) time.Time { return value.createdAt },
		time.Time.Compare,
	)
	earlier := record{createdAt: time.Unix(1, 0)}
	later := record{createdAt: time.Unix(2, 0)}
	if got := byCreatedAt(earlier, later); got >= 0 {
		t.Fatalf("time.Time On comparator result = %d, want negative", got)
	}

	primary := comparator.By(func(value record) int { return value.rank })
	var thenByValue func(func(record) string) comparator.Func[record] = primary.ThenBy
	if got := thenByValue(func(value record) string { return value.name })(
		record{rank: 1, name: "a"},
		record{rank: 1, name: "b"},
	); got >= 0 {
		t.Fatalf("ThenBy method value result = %d, want negative", got)
	}
	var thenByExpression func(comparator.Func[record], func(record) string) comparator.Func[record] = comparator.Func[record].ThenBy
	if got := thenByExpression(primary, func(value record) string { return value.name })(
		record{rank: 1, name: "a"},
		record{rank: 1, name: "b"},
	); got >= 0 {
		t.Fatalf("ThenBy method expression result = %d, want negative", got)
	}

	var thenByDescendingValue func(func(record) string) comparator.Func[record] = primary.ThenByDescending
	if got := thenByDescendingValue(func(value record) string { return value.name })(
		record{rank: 1, name: "a"},
		record{rank: 1, name: "b"},
	); got <= 0 {
		t.Fatalf("ThenByDescending method value result = %d, want positive", got)
	}

	var thenOnValue func(func(record) time.Time, comparator.Func[time.Time]) comparator.Func[record] = primary.ThenOn
	if got := thenOnValue(func(value record) time.Time { return value.createdAt }, time.Time.Compare)(earlier, later); got >= 0 {
		t.Fatalf("ThenOn method value result = %d, want negative", got)
	}
	var thenOnExpression func(comparator.Func[record], func(record) time.Time, comparator.Func[time.Time]) comparator.Func[record] = comparator.Func[record].ThenOn
	if got := thenOnExpression(primary, func(value record) time.Time { return value.createdAt }, time.Time.Compare)(earlier, later); got >= 0 {
		t.Fatalf("ThenOn method expression result = %d, want negative", got)
	}

	var thenOnDescendingValue func(func(record) time.Time, comparator.Func[time.Time]) comparator.Func[record] = primary.ThenOnDescending
	if got := thenOnDescendingValue(func(value record) time.Time { return value.createdAt }, time.Time.Compare)(earlier, later); got <= 0 {
		t.Fatalf("ThenOnDescending method value result = %d, want positive", got)
	}

	mixedKeyTypes := primary.
		ThenBy[string](func(value record) string { return value.name }).
		ThenOn(func(value record) time.Time { return value.createdAt }, time.Time.Compare).
		ThenByDescending(func(value record) int { return value.id })
	_ = mixedKeyTypes
}

func TestImmutableComparatorConcurrentEvaluation(t *testing.T) {
	compare := comparator.By(func(value record) int { return value.rank }).
		ThenByDescending(func(value record) string { return value.name })
	left := record{rank: 1, name: "z"}
	right := record{rank: 1, name: "a"}

	const goroutines = 16
	const evaluations = 1_000
	errors := make(chan int, goroutines)
	var wait sync.WaitGroup
	for range goroutines {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range evaluations {
				if got := compare(left, right); got >= 0 {
					errors <- got
					return
				}
			}
		}()
	}
	wait.Wait()
	close(errors)
	for got := range errors {
		t.Fatalf("concurrent comparator result = %d, want negative", got)
	}
}

type panicMarker struct {
	label string
}

func requirePanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s did not panic", name)
		}
	}()
	fn()
}

func requirePanicValue(t *testing.T, want any, fn func()) {
	t.Helper()
	defer func() {
		if got := recover(); got != want {
			t.Fatalf("panic value = %#v, want %#v", got, want)
		}
	}()
	fn()
}

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}

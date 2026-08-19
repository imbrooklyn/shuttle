package predicate_test

import (
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/imbrooklyn/shuttle/optional"
	"github.com/imbrooklyn/shuttle/predicate"
	"github.com/imbrooklyn/shuttle/stream"
)

func TestZeroFuncIsAnOrdinaryNilFunction(t *testing.T) {
	var zero predicate.Func[int]
	if zero != nil {
		t.Fatal("zero Func is not nil")
	}

	not := zero.Not()
	and := zero.And(predicate.Always[int](true))
	or := zero.Or(predicate.Always[int](false))
	for name, evaluation := range map[string]func(){
		"direct": func() { zero(1) },
		"Not":    func() { not(1) },
		"And":    func() { and(1) },
		"Or":     func() { or(1) },
	} {
		t.Run(name, func(t *testing.T) {
			requirePanic(t, evaluation)
		})
	}
}

func TestNotTruthTableAndExactInvocation(t *testing.T) {
	for _, result := range []bool{false, true} {
		calls := 0
		base := predicate.Func[int](func(int) bool {
			calls++
			return result
		})
		negated := base.Not()
		if calls != 0 {
			t.Fatal("Not evaluated its receiver during construction")
		}
		if got := negated(7); got != !result {
			t.Fatalf("Not result = %v, want %v", got, !result)
		}
		if calls != 1 {
			t.Fatalf("Not receiver calls = %d, want 1", calls)
		}
	}
}

func TestAndOrTruthTables(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		a := mask&1 != 0
		b := mask&2 != 0
		c := mask&4 != 0
		and := predicate.Always[int](a).And(
			predicate.Always[int](b),
			predicate.Always[int](c),
		)
		or := predicate.Always[int](a).Or(
			predicate.Always[int](b),
			predicate.Always[int](c),
		)
		if got, want := and(0), a && b && c; got != want {
			t.Fatalf("And(%v, %v, %v) = %v, want %v", a, b, c, got, want)
		}
		if got, want := or(0), a || b || c; got != want {
			t.Fatalf("Or(%v, %v, %v) = %v, want %v", a, b, c, got, want)
		}
	}
}

func TestAndOrWithoutOthersCallReceiverOnce(t *testing.T) {
	andCalls := 0
	andReceiver := predicate.Func[int](func(int) bool {
		andCalls++
		return true
	})
	if !andReceiver.And()(0) || andCalls != 1 {
		t.Fatalf("And without others = true with %d calls, want one call", andCalls)
	}

	orCalls := 0
	orReceiver := predicate.Func[int](func(int) bool {
		orCalls++
		return false
	})
	if orReceiver.Or()(0) || orCalls != 1 {
		t.Fatalf("Or without others = false with %d calls, want one call", orCalls)
	}
}

func TestAndOrOrderCountsAndShortCircuit(t *testing.T) {
	newLogged := func(log *[]string, name string, result bool) predicate.Func[int] {
		return func(int) bool {
			*log = append(*log, name)
			return result
		}
	}

	var andLog []string
	and := newLogged(&andLog, "receiver", true).And(
		newLogged(&andLog, "second", true),
		newLogged(&andLog, "third", false),
		newLogged(&andLog, "skipped", true),
	)
	if andLog != nil {
		t.Fatal("And evaluated predicates during construction")
	}
	if and(1) {
		t.Fatal("And returned true after a false predicate")
	}
	if want := []string{"receiver", "second", "third"}; !slices.Equal(andLog, want) {
		t.Fatalf("And order = %v, want %v", andLog, want)
	}

	var orLog []string
	or := newLogged(&orLog, "receiver", false).Or(
		newLogged(&orLog, "second", false),
		newLogged(&orLog, "third", true),
		newLogged(&orLog, "skipped", false),
	)
	if orLog != nil {
		t.Fatal("Or evaluated predicates during construction")
	}
	if !or(1) {
		t.Fatal("Or returned false after a true predicate")
	}
	if want := []string{"receiver", "second", "third"}; !slices.Equal(orLog, want) {
		t.Fatalf("Or order = %v, want %v", orLog, want)
	}
}

func TestReachedAndSkippedNilPredicates(t *testing.T) {
	var nilPredicate predicate.Func[int]

	if predicate.Always[int](false).And(nilPredicate)(0) {
		t.Fatal("short-circuited And returned true")
	}
	if !predicate.Always[int](true).Or(nilPredicate)(0) {
		t.Fatal("short-circuited Or returned false")
	}

	requirePanic(t, func() {
		predicate.Always[int](true).And(nilPredicate)(0)
	})
	requirePanic(t, func() {
		predicate.Always[int](false).Or(nilPredicate)(0)
	})
}

func TestCompositionPropagatesPanicsUnchanged(t *testing.T) {
	sentinel := errors.New("sentinel")
	panicking := predicate.Func[int](func(int) bool { panic(sentinel) })

	for name, evaluation := range map[string]func(){
		"Not": func() { panicking.Not()(0) },
		"And": func() { predicate.Always[int](true).And(panicking)(0) },
		"Or":  func() { predicate.Always[int](false).Or(panicking)(0) },
	} {
		t.Run(name, func(t *testing.T) {
			if got := recoveredPanic(t, evaluation); got != sentinel {
				t.Fatalf("panic = %v, want sentinel", got)
			}
		})
	}
}

func TestAndOrSnapshotVariadicDescriptors(t *testing.T) {
	andOthers := []predicate.Func[int]{predicate.Equal(3)}
	and := predicate.Always[int](true).And(andOthers...)
	andOthers[0] = nil
	if !and(3) {
		t.Fatal("And observed replacement in caller's variadic slice")
	}

	orOthers := []predicate.Func[int]{predicate.Equal(3)}
	or := predicate.Always[int](false).Or(orOthers...)
	orOthers[0] = nil
	if !or(3) {
		t.Fatal("Or observed replacement in caller's variadic slice")
	}

	threshold := 1
	captured := predicate.Func[int](func(value int) bool { return value > threshold })
	composed := predicate.Always[int](true).And(captured)
	threshold = 3
	if composed(2) {
		t.Fatal("composition copied callback-captured state")
	}
}

func TestPredicatesDoNotCacheEvaluationResults(t *testing.T) {
	calls := 0
	changing := predicate.Func[int](func(int) bool {
		calls++
		return calls%2 == 0
	})
	composed := changing.Not()
	if !composed(0) {
		t.Fatal("first evaluation = false, want true")
	}
	if composed(0) {
		t.Fatal("second evaluation = true, want false")
	}
	if calls != 2 {
		t.Fatalf("callback calls = %d, want 2", calls)
	}
}

func TestAlways(t *testing.T) {
	if !predicate.Always[int](true)(0) {
		t.Fatal("Always(true) returned false")
	}
	if predicate.Always[string](false)("ignored") {
		t.Fatal("Always(false) returned true")
	}
}

func TestEqualUsesGoEquality(t *testing.T) {
	if !predicate.Equal(0)(0) || predicate.Equal(0)(1) {
		t.Fatal("Equal failed for integer zero")
	}
	if !predicate.Equal("")("") || predicate.Equal("")("x") {
		t.Fatal("Equal failed for string zero")
	}
	var nilPointer *int
	if !predicate.Equal(nilPointer)(nil) {
		t.Fatal("Equal failed for nil pointer zero")
	}

	dynamicSlice := any([]int{1})
	equalDynamicSlice := predicate.Equal[any](dynamicSlice)
	requirePanic(t, func() { equalDynamicSlice(dynamicSlice) })
}

func TestEqualFuncOrderCountAndNilCallback(t *testing.T) {
	want := []int{1, 2}
	current := []int{1, 2}
	calls := 0
	equal := predicate.EqualFunc(want, func(gotCurrent, gotWant []int) bool {
		calls++
		if len(gotCurrent) > 0 && &gotCurrent[0] != &current[0] {
			t.Fatal("EqualFunc did not pass current value first")
		}
		if len(gotWant) > 0 && &gotWant[0] != &want[0] {
			t.Fatal("EqualFunc did not pass want second")
		}
		return slices.Equal(gotCurrent, gotWant)
	})
	if !equal(current) {
		t.Fatal("EqualFunc returned false")
	}
	if calls != 1 {
		t.Fatalf("EqualFunc calls = %d, want 1", calls)
	}

	var nilEqual func(int, int) bool
	nilPredicate := predicate.EqualFunc(1, nilEqual)
	requirePanic(t, func() { nilPredicate(1) })
}

func TestOnOrderCountAndPanicPropagation(t *testing.T) {
	type record struct{ score int }
	var order []string
	projectCalls, predicateCalls := 0, 0
	projectedEven := predicate.On(
		func(value record) int {
			order = append(order, "project")
			projectCalls++
			return value.score
		},
		predicate.Func[int](func(value int) bool {
			order = append(order, "predicate")
			predicateCalls++
			return value%2 == 0
		}),
	)
	if order != nil {
		t.Fatal("On evaluated callbacks during construction")
	}
	if !projectedEven(record{score: 4}) {
		t.Fatal("On returned false")
	}
	if projectCalls != 1 || predicateCalls != 1 {
		t.Fatalf("On calls = project %d predicate %d, want 1 each", projectCalls, predicateCalls)
	}
	if want := []string{"project", "predicate"}; !slices.Equal(order, want) {
		t.Fatalf("On order = %v, want %v", order, want)
	}

	projectPanic := errors.New("project")
	predicateAfterProject := 0
	projectFails := predicate.On(
		func(record) int { panic(projectPanic) },
		predicate.Func[int](func(int) bool { predicateAfterProject++; return true }),
	)
	if got := recoveredPanic(t, func() { projectFails(record{}) }); got != projectPanic {
		t.Fatalf("project panic = %v, want %v", got, projectPanic)
	}
	if predicateAfterProject != 0 {
		t.Fatal("On invoked predicate after project panicked")
	}

	predicatePanic := errors.New("predicate")
	predicateFails := predicate.On(
		func(record) int { return 1 },
		predicate.Func[int](func(int) bool { panic(predicatePanic) }),
	)
	if got := recoveredPanic(t, func() { predicateFails(record{}) }); got != predicatePanic {
		t.Fatalf("predicate panic = %v, want %v", got, predicatePanic)
	}

	var nilProject func(record) int
	nilProjectPredicate := predicate.On(nilProject, predicate.Always[int](true))
	requirePanic(t, func() { nilProjectPredicate(record{}) })

	nilPredicateProjectCalls := 0
	var nilPredicate predicate.Func[int]
	nilProjectedPredicate := predicate.On(
		func(record) int {
			nilPredicateProjectCalls++
			return 1
		},
		nilPredicate,
	)
	requirePanic(t, func() { nilProjectedPredicate(record{}) })
	if nilPredicateProjectCalls != 1 {
		t.Fatalf("project calls before reached nil predicate = %d, want 1", nilPredicateProjectCalls)
	}
}

type namedChannel chan int
type namedFunction func()
type namedMap map[string]int
type namedPointer *int
type namedSlice []byte
type namedUnsafePointer unsafe.Pointer

type marked interface {
	mark()
}

type marker struct{}

func (*marker) mark() {}

func TestIsNilSupportsEveryNilableKind(t *testing.T) {
	var nilInterface any
	var nilChannel chan int
	var nilFunction func()
	var nilMap map[string]int
	var nilPointer *int
	var nilSlice []int
	var nilUnsafePointer unsafe.Pointer
	var typedNilInterface marked = (*marker)(nil)

	values := []struct {
		name  string
		value any
	}{
		{name: "nil interface", value: nilInterface},
		{name: "channel", value: nilChannel},
		{name: "function", value: nilFunction},
		{name: "map", value: nilMap},
		{name: "pointer", value: nilPointer},
		{name: "slice", value: nilSlice},
		{name: "unsafe pointer", value: nilUnsafePointer},
		{name: "typed nil through interface", value: typedNilInterface},
	}
	for _, test := range values {
		t.Run(test.name, func(t *testing.T) {
			if !predicate.IsNil(test.value) {
				t.Fatalf("IsNil(%T) = false", test.value)
			}
			if predicate.IsNotNil(test.value) {
				t.Fatalf("IsNotNil(%T) = true", test.value)
			}
		})
	}

	if !predicate.IsNil(nilChannel) || !predicate.IsNil(nilFunction) ||
		!predicate.IsNil(nilMap) || !predicate.IsNil(nilPointer) ||
		!predicate.IsNil(nilSlice) || !predicate.IsNil(nilUnsafePointer) {
		t.Fatal("IsNil failed with a statically typed nilable value")
	}
}

func TestIsNilSupportsNamedNilableTypes(t *testing.T) {
	var channel namedChannel
	var function namedFunction
	var mapping namedMap
	var pointer namedPointer
	var slice namedSlice
	var unsafePointer namedUnsafePointer

	if !predicate.IsNil(channel) || !predicate.IsNil(function) ||
		!predicate.IsNil(mapping) || !predicate.IsNil(pointer) ||
		!predicate.IsNil(slice) || !predicate.IsNil(unsafePointer) {
		t.Fatal("IsNil failed for a named nilable type")
	}
}

func TestIsNilRejectsNonNilAndNonNilableValues(t *testing.T) {
	channel := make(chan int)
	defer close(channel)
	function := func() {}
	mapping := map[string]int{}
	number := 1
	pointer := &number
	slice := make([]int, 0)
	unsafePointer := unsafe.Pointer(pointer)

	values := []any{
		channel,
		function,
		mapping,
		pointer,
		slice,
		unsafePointer,
		0,
		"",
		false,
		struct{}{},
		[0]int{},
	}
	for _, value := range values {
		if predicate.IsNil(value) {
			t.Fatalf("IsNil(%T) = true", value)
		}
		if !predicate.IsNotNil(value) {
			t.Fatalf("IsNotNil(%T) = false", value)
		}
	}
}

func TestIsNotNilIsAlwaysExactNegation(t *testing.T) {
	var nilPointer *int
	value := 1
	values := []any{nil, nilPointer, []byte(nil), []byte{}, 0, "", &value}
	for _, current := range values {
		if got, want := predicate.IsNotNil(current), !predicate.IsNil(current); got != want {
			t.Fatalf("IsNotNil(%T) = %v, want %v", current, got, want)
		}
	}
}

type scored int

func (value scored) Positive() bool { return value > 0 }

func TestInferenceMethodValuesAndMethodExpressions(t *testing.T) {
	inferred := predicate.Equal(3)
	explicit := predicate.Equal[int](3)
	if !inferred(3) || !explicit(3) {
		t.Fatal("Equal inference or explicit instantiation failed")
	}

	var reverseInferred predicate.Func[*int] = predicate.IsNil
	var unnamed func(*int) bool = predicate.IsNotNil
	if !reverseInferred(nil) || unnamed(nil) {
		t.Fatal("generic function reverse inference failed")
	}

	type holder struct{ pointer *int }
	on := predicate.On(func(value holder) *int { return value.pointer }, predicate.IsNotNil)
	if on(holder{}) {
		t.Fatal("On reverse inference returned true for nil projection")
	}

	base := predicate.Func[int](func(value int) bool { return value > 0 })
	notValue := base.Not
	andValue := base.And
	orValue := base.Or
	if notValue()(1) || !andValue(predicate.Equal(1))(1) || !orValue(predicate.Equal(0))(1) {
		t.Fatal("Func method value produced an incorrect result")
	}

	notExpression := predicate.Func[int].Not
	andExpression := predicate.Func[int].And
	orExpression := predicate.Func[int].Or
	if notExpression(base)(1) || !andExpression(base, predicate.Equal(1))(1) ||
		!orExpression(base, predicate.Equal(0))(1) {
		t.Fatal("Func method expression produced an incorrect result")
	}

	var ordinaryMethodExpression predicate.Func[scored] = scored.Positive
	if !ordinaryMethodExpression(1) || ordinaryMethodExpression(0) {
		t.Fatal("ordinary method expression adaptation failed")
	}

	var namedToUnnamed func(int) bool = base
	if !namedToUnnamed(1) {
		t.Fatal("Func was not assignable to func(int) bool")
	}
}

func TestOptionalAndStreamFilterInteroperability(t *testing.T) {
	equalTwo := predicate.Equal(2)
	if got := optional.Some(2).Filter(equalTwo); got.IsNone() || got.Must() != 2 {
		t.Fatalf("Optional.Filter result = %v", got)
	}
	if got := stream.Of(1, 2, 3, 2).Filter(equalTwo).Collect(); !slices.Equal(got, []int{2, 2}) {
		t.Fatalf("Stream.Filter result = %v", got)
	}

	var nilPointer *int
	if optional.Some(nilPointer).Filter(predicate.IsNil).IsNone() {
		t.Fatal("Optional.Filter rejected typed nil")
	}
	if got := stream.Of(nilPointer, new(int)).Filter(predicate.IsNil).Count(); got != 1 {
		t.Fatalf("Stream.Filter IsNil count = %d, want 1", got)
	}
}

func TestConcurrentEvaluationOfImmutablePredicates(t *testing.T) {
	composed := predicate.Func[int](func(value int) bool { return value >= 0 }).
		And(
			predicate.Func[int](func(value int) bool { return value%2 == 0 }),
			predicate.Func[int](func(value int) bool { return value < 1000 }),
		).
		Or(predicate.Equal(-1))

	const goroutines = 16
	const evaluations = 2000
	var mismatches atomic.Int64
	var wait sync.WaitGroup
	wait.Add(goroutines)
	for worker := range goroutines {
		go func() {
			defer wait.Done()
			for index := range evaluations {
				value := (worker*evaluations+index)%1200 - 1
				want := (value >= 0 && value%2 == 0 && value < 1000) || value == -1
				if composed(value) != want {
					mismatches.Add(1)
				}
			}
		}()
	}
	wait.Wait()
	if got := mismatches.Load(); got != 0 {
		t.Fatalf("concurrent mismatches = %d", got)
	}
}

func TestCapturedMutableStateRemainsCallerOwned(t *testing.T) {
	threshold := 1
	greaterThanThreshold := predicate.Func[int](func(value int) bool {
		return value > threshold
	})
	if !greaterThanThreshold(2) {
		t.Fatal("initial captured state was not observed")
	}
	threshold = 3
	if greaterThanThreshold(2) {
		t.Fatal("updated captured state was not observed")
	}

	var synchronizedThreshold atomic.Int64
	synchronizedThreshold.Store(3)
	concurrencySafe := predicate.Func[int](func(value int) bool {
		return int64(value) > synchronizedThreshold.Load()
	})
	if !concurrencySafe(4) {
		t.Fatal("synchronized captured state was not observed")
	}
}

func requirePanic(t *testing.T, fn func()) {
	t.Helper()
	recoveredPanic(t, fn)
}

func recoveredPanic(t *testing.T, fn func()) (value any) {
	t.Helper()
	didPanic := false
	func() {
		defer func() {
			value = recover()
			didPanic = value != nil
		}()
		fn()
	}()
	if !didPanic {
		t.Fatal("evaluation did not panic")
	}
	return value
}

package predicate_test

import (
	"testing"

	"github.com/imbrooklyn/shuttle/optional"
	"github.com/imbrooklyn/shuttle/predicate"
)

var allocationBoolSink bool
var allocationOptionalSink optional.Optional[int]

type allocationRecord struct {
	value int
}

func allocationPositive(value int) bool {
	return value > 0
}

func allocationEven(value int) bool {
	return value%2 == 0
}

func allocationEqual(a, b int) bool {
	return a == b
}

func allocationProject(value allocationRecord) int {
	return value.value
}

func TestCorePredicateEvaluationAllocations(t *testing.T) {
	base := predicate.Func[int](allocationPositive)
	not := base.Not()
	and := base.And(predicate.Func[int](allocationEven), predicate.Equal(2))
	or := predicate.Always[int](false).Or(predicate.Always[int](false), predicate.Equal(2))
	always := predicate.Always[int](true)
	equal := predicate.Equal(2)
	equalFunc := predicate.EqualFunc(2, allocationEqual)
	on := predicate.On(allocationProject, predicate.Equal(2))
	var nilPointer *int

	assertZeroEvaluationAllocations(t, "Not", func() {
		allocationBoolSink = not(2)
	})
	assertZeroEvaluationAllocations(t, "And", func() {
		allocationBoolSink = and(2)
	})
	assertZeroEvaluationAllocations(t, "Or", func() {
		allocationBoolSink = or(2)
	})
	assertZeroEvaluationAllocations(t, "Always", func() {
		allocationBoolSink = always(2)
	})
	assertZeroEvaluationAllocations(t, "Equal", func() {
		allocationBoolSink = equal(2)
	})
	assertZeroEvaluationAllocations(t, "EqualFunc", func() {
		allocationBoolSink = equalFunc(2)
	})
	assertZeroEvaluationAllocations(t, "On", func() {
		allocationBoolSink = on(allocationRecord{value: 2})
	})
	assertZeroEvaluationAllocations(t, "IsNil", func() {
		allocationBoolSink = predicate.IsNil(nilPointer)
	})
	assertZeroEvaluationAllocations(t, "IsNotNil", func() {
		allocationBoolSink = predicate.IsNotNil(nilPointer)
	})

	some := optional.Some(2)
	assertZeroEvaluationAllocations(t, "Optional.Filter interoperability", func() {
		allocationOptionalSink = some.Filter(equal)
	})
}

func assertZeroEvaluationAllocations(t *testing.T, name string, evaluate func()) {
	t.Helper()
	if got := testing.AllocsPerRun(1000, evaluate); got != 0 {
		t.Fatalf("%s allocations per evaluation = %v, want 0", name, got)
	}
}

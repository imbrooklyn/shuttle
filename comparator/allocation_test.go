package comparator_test

import (
	"testing"

	"github.com/imbrooklyn/shuttle/comparator"
)

type allocationValue struct {
	primary   int
	secondary int
}

var allocationResult int

func TestComparatorEvaluationAllocations(t *testing.T) {
	left := allocationValue{primary: 1, secondary: 4}
	right := allocationValue{primary: 2, secondary: 3}

	ordered := comparator.Ordered[int]()
	by := comparator.By(func(value allocationValue) int { return value.primary })
	byDescending := comparator.ByDescending(func(value allocationValue) int { return value.primary })
	on := comparator.On(
		func(value allocationValue) int { return value.secondary },
		comparator.Ordered[int](),
	)
	onDescending := comparator.OnDescending(
		func(value allocationValue) int { return value.secondary },
		comparator.Ordered[int](),
	)
	reversed := by.Reverse()
	combined := by.Then(
		comparator.ByDescending(func(value allocationValue) int { return value.secondary }),
	)
	equalPrimary := comparator.Func[allocationValue](func(allocationValue, allocationValue) int { return 0 })
	thenBy := equalPrimary.ThenBy(func(value allocationValue) int { return value.secondary })
	thenByDescending := equalPrimary.ThenByDescending(func(value allocationValue) int { return value.secondary })
	thenOn := equalPrimary.ThenOn(
		func(value allocationValue) int { return value.secondary },
		comparator.Ordered[int](),
	)
	thenOnDescending := equalPrimary.ThenOnDescending(
		func(value allocationValue) int { return value.secondary },
		comparator.Ordered[int](),
	)

	tests := []struct {
		name     string
		evaluate func()
	}{
		{name: "Ordered", evaluate: func() { allocationResult = ordered(left.primary, right.primary) }},
		{name: "By", evaluate: func() { allocationResult = by(left, right) }},
		{name: "ByDescending", evaluate: func() { allocationResult = byDescending(left, right) }},
		{name: "On", evaluate: func() { allocationResult = on(left, right) }},
		{name: "OnDescending", evaluate: func() { allocationResult = onDescending(left, right) }},
		{name: "Reverse", evaluate: func() { allocationResult = reversed(left, right) }},
		{name: "Then", evaluate: func() { allocationResult = combined(left, right) }},
		{name: "ThenBy", evaluate: func() { allocationResult = thenBy(left, right) }},
		{name: "ThenByDescending", evaluate: func() { allocationResult = thenByDescending(left, right) }},
		{name: "ThenOn", evaluate: func() { allocationResult = thenOn(left, right) }},
		{name: "ThenOnDescending", evaluate: func() { allocationResult = thenOnDescending(left, right) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if allocations := testing.AllocsPerRun(1_000, test.evaluate); allocations != 0 {
				t.Fatalf("allocations per evaluation = %v, want 0", allocations)
			}
		})
	}
}

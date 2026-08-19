package optional_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/imbrooklyn/shuttle/optional"
)

func TestStateAndConstructors(t *testing.T) {
	zero := 0
	ptr := &zero
	var nilSlice []int
	var nilPtr *int

	tests := []struct {
		name    string
		value   optional.Optional[any]
		present bool
		want    any
	}{
		{name: "zero value", value: optional.Optional[any]{}},
		{name: "None", value: optional.None[any]()},
		{name: "Some zero", value: optional.Some[any](0), present: true, want: 0},
		{name: "Some empty string", value: optional.Some[any](""), present: true, want: ""},
		{name: "Some false", value: optional.Some[any](false), present: true, want: false},
		{name: "Some nil slice", value: optional.Some[any](nilSlice), present: true, want: nilSlice},
		{name: "Some nil pointer", value: optional.Some[any](nilPtr), present: true, want: nilPtr},
		{name: "Some nil interface", value: optional.Some[any](nil), present: true, want: nil},
		{name: "Of true", value: optional.Of[any](false, true), present: true, want: false},
		{name: "Of false", value: optional.Of[any](99, false)},
		{name: "FromPtr value", value: optional.Some[any](optional.FromPtr(ptr).Must()), present: true, want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.value.IsSome() != test.present {
				t.Fatalf("IsSome() = %v, want %v", test.value.IsSome(), test.present)
			}
			if test.value.IsNone() == test.present {
				t.Fatalf("IsNone() = %v, want %v", test.value.IsNone(), !test.present)
			}
			got, ok := test.value.Value()
			if ok != test.present {
				t.Fatalf("Value presence = %v, want %v", ok, test.present)
			}
			if test.present && !equalAny(got, test.want) {
				t.Fatalf("Value() = %#v, want %#v", got, test.want)
			}
		})
	}

	if optional.FromPtr[int](nil).IsSome() {
		t.Fatal("FromPtr(nil) returned Some")
	}
	*ptr = 7
	if got := optional.FromPtr(ptr).Must(); got != 7 {
		t.Fatalf("FromPtr value = %d, want 7", got)
	}
	fromPtr := optional.FromPtr(ptr)
	*ptr = 8
	if got := fromPtr.Must(); got != 7 {
		t.Fatalf("FromPtr did not copy value: got %d", got)
	}
}

func TestGenericMethodValuesAndExpressions(t *testing.T) {
	var methodValue func(func(int) string) optional.Optional[string] = optional.Some(2).Map
	if got := methodValue(func(value int) string { return strings.Repeat("x", value) }).Must(); got != "xx" {
		t.Fatalf("Map method value = %q", got)
	}

	methodExpression := optional.Optional[int].Map[string]
	if got := methodExpression(optional.Some(3), func(value int) string {
		return strings.Repeat("y", value)
	}).Must(); got != "yyy" {
		t.Fatalf("Map method expression = %q", got)
	}
}

func equalAny(a, b any) bool {
	switch a := a.(type) {
	case []int:
		b, ok := b.([]int)
		return ok && a == nil && b == nil
	case *int:
		b, ok := b.(*int)
		return ok && a == b
	default:
		return a == b
	}
}

func TestExtraction(t *testing.T) {
	some := optional.Some(4)
	none := optional.None[int]()

	if got := some.OrZero(); got != 4 {
		t.Fatalf("Some.OrZero() = %d", got)
	}
	if got := none.OrZero(); got != 0 {
		t.Fatalf("None.OrZero() = %d", got)
	}
	if got := some.OrElse(9); got != 4 {
		t.Fatalf("Some.OrElse() = %d", got)
	}
	if got := none.OrElse(9); got != 9 {
		t.Fatalf("None.OrElse() = %d", got)
	}

	calls := 0
	if got := some.OrElseGet(func() int { calls++; return 9 }); got != 4 || calls != 0 {
		t.Fatalf("Some.OrElseGet() = %d, calls = %d", got, calls)
	}
	if got := none.OrElseGet(func() int { calls++; return 9 }); got != 9 || calls != 1 {
		t.Fatalf("None.OrElseGet() = %d, calls = %d", got, calls)
	}
	if got := some.Must(); got != 4 {
		t.Fatalf("Some.Must() = %d", got)
	}
	assertPanics(t, "None.Must", func() { none.Must() })
}

func TestPtrReturnsIndependentCopies(t *testing.T) {
	some := optional.Some([]int{1, 2})
	first := some.Ptr()
	second := some.Ptr()
	if first == nil || second == nil || first == second {
		t.Fatalf("Ptr results are not independent: %p %p", first, second)
	}
	*first = []int{9}
	if got := len(*second); got != 2 {
		t.Fatalf("assigning first pointer changed second: len = %d", got)
	}
	if got := len(some.Must()); got != 2 {
		t.Fatalf("assigning pointer changed Optional: len = %d", got)
	}
	(*second)[0] = 7
	if got := some.Must()[0]; got != 7 {
		t.Fatalf("Ptr did not preserve shallow element aliases: got %d", got)
	}
	if optional.None[int]().Ptr() != nil {
		t.Fatal("None.Ptr() is non-nil")
	}
}

func TestTransformationsAndCallbackSelection(t *testing.T) {
	t.Run("Some chain", func(t *testing.T) {
		calls := map[string]int{}
		got := optional.Some(3).
			Map(func(value int) string { calls["map"]++; return strings.Repeat("x", value) }).
			FlatMap(func(value string) optional.Optional[int] { calls["flatmap"]++; return optional.Some(len(value)) }).
			Filter(func(value int) bool { calls["filter"]++; return value == 3 }).
			Inspect(func(int) { calls["inspect"]++ })
		if got.Must() != 3 {
			t.Fatalf("chain result = %v", got)
		}
		for _, name := range []string{"map", "flatmap", "filter", "inspect"} {
			if calls[name] != 1 {
				t.Fatalf("%s calls = %d, want 1", name, calls[name])
			}
		}
	})

	t.Run("None skips callbacks", func(t *testing.T) {
		var nilMap func(int) string
		var nilFlatMap func(int) optional.Optional[string]
		var nilPredicate func(int) bool
		var nilInspect func(int)
		if optional.None[int]().Map(nilMap).IsSome() {
			t.Fatal("Map on None returned Some")
		}
		if optional.None[int]().FlatMap(nilFlatMap).IsSome() {
			t.Fatal("FlatMap on None returned Some")
		}
		if optional.None[int]().Filter(nilPredicate).IsSome() {
			t.Fatal("Filter on None returned Some")
		}
		if optional.None[int]().Inspect(nilInspect).IsSome() {
			t.Fatal("Inspect on None returned Some")
		}
	})

	if optional.Some(1).Filter(func(int) bool { return false }).IsSome() {
		t.Fatal("Filter retained rejected value")
	}
	if got := optional.Some(1).Map(func(int) *int { return nil }); !got.IsSome() || got.Must() != nil {
		t.Fatalf("Map lost present nil: %v", got)
	}
}

func TestCompositionAndEquality(t *testing.T) {
	someCalls, noneCalls := 0, 0
	if got := optional.Some(2).Match(
		func(v int) string { someCalls++; return strings.Repeat("x", v) },
		func() string { noneCalls++; return "none" },
	); got != "xx" || someCalls != 1 || noneCalls != 0 {
		t.Fatalf("Some.Match = %q, calls = (%d,%d)", got, someCalls, noneCalls)
	}
	if got := optional.None[int]().Match(
		func(int) string { someCalls++; return "some" },
		func() string { noneCalls++; return "none" },
	); got != "none" || someCalls != 1 || noneCalls != 1 {
		t.Fatalf("None.Match = %q, calls = (%d,%d)", got, someCalls, noneCalls)
	}

	combineCalls := 0
	combined := optional.Some(2).ZipWith(optional.Some("abc"), func(a int, b string) int {
		combineCalls++
		return a + len(b)
	})
	if combined.Must() != 5 || combineCalls != 1 {
		t.Fatalf("ZipWith result = %v, calls = %d", combined, combineCalls)
	}
	if optional.Some(2).ZipWith(optional.None[string](), func(int, string) int {
		combineCalls++
		return 0
	}).IsSome() || combineCalls != 1 {
		t.Fatalf("ZipWith invoked callback for None, calls = %d", combineCalls)
	}

	if got := optional.Some(1).Or(optional.Some(2)).Must(); got != 1 {
		t.Fatalf("Some.Or = %d", got)
	}
	if got := optional.None[int]().Or(optional.Some(2)).Must(); got != 2 {
		t.Fatalf("None.Or = %d", got)
	}
	orCalls := 0
	if got := optional.Some(1).OrGet(func() optional.Optional[int] { orCalls++; return optional.Some(2) }).Must(); got != 1 || orCalls != 0 {
		t.Fatalf("Some.OrGet = %d, calls = %d", got, orCalls)
	}
	if got := optional.None[int]().OrGet(func() optional.Optional[int] { orCalls++; return optional.Some(2) }).Must(); got != 2 || orCalls != 1 {
		t.Fatalf("None.OrGet = %d, calls = %d", got, orCalls)
	}

	if optional.Flatten(optional.None[optional.Optional[int]]()).IsSome() {
		t.Fatal("Flatten outer None returned Some")
	}
	if optional.Flatten(optional.Some(optional.None[int]())).IsSome() {
		t.Fatal("Flatten inner None returned Some")
	}
	if got := optional.Flatten(optional.Some(optional.Some(8))).Must(); got != 8 {
		t.Fatalf("Flatten = %d", got)
	}

	equalityTests := []struct {
		name string
		a, b optional.Optional[int]
		want bool
	}{
		{name: "none none", a: optional.None[int](), b: optional.None[int](), want: true},
		{name: "some equal", a: optional.Some(1), b: optional.Some(1), want: true},
		{name: "some unequal", a: optional.Some(1), b: optional.Some(2)},
		{name: "presence mismatch", a: optional.None[int](), b: optional.Some(0)},
	}
	for _, test := range equalityTests {
		t.Run(test.name, func(t *testing.T) {
			if got := optional.Equal(test.a, test.b); got != test.want {
				t.Fatalf("Equal = %v, want %v", got, test.want)
			}
			if got := test.a == test.b; got != test.want {
				t.Fatalf("direct equality = %v, want %v", got, test.want)
			}
		})
	}

	equalCalls := 0
	equal := func(a, b []int) bool { equalCalls++; return len(a) == len(b) }
	if !optional.EqualFunc(optional.None[[]int](), optional.None[[]int](), equal) || equalCalls != 0 {
		t.Fatalf("EqualFunc(None,None), calls = %d", equalCalls)
	}
	if optional.EqualFunc(optional.Some([]int{}), optional.None[[]int](), equal) || equalCalls != 0 {
		t.Fatalf("EqualFunc presence mismatch, calls = %d", equalCalls)
	}
	if !optional.EqualFunc(optional.Some([]int{1}), optional.Some([]int{2}), equal) || equalCalls != 1 {
		t.Fatalf("EqualFunc Some values, calls = %d", equalCalls)
	}
}

func TestComparabilityAndDynamicComparison(t *testing.T) {
	values := map[optional.Optional[int]]string{
		optional.None[int](): "none",
		optional.Some(0):     "zero",
	}
	if values[optional.Some(0)] != "zero" {
		t.Fatal("Optional[int] is not usable as a comparable map key")
	}

	none := optional.None[any]()
	presentSlice := optional.Some[any]([]int{1})
	if none == presentSlice {
		t.Fatal("presence mismatch compared equal")
	}
	assertPanics(t, "dynamic non-comparable comparison", func() {
		_ = presentSlice == optional.Some[any]([]int{1})
	})
}

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s did not panic", name)
		}
	}()
	fn()
}

func FuzzOptionalJSON(f *testing.F) {
	f.Add([]byte("null"))
	f.Add([]byte("0"))
	f.Add([]byte(`{"a":[1,true,null]}`))
	f.Add([]byte("{"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		value := optional.Some[any]("unchanged")
		err := json.Unmarshal(data, &value)
		if value.IsSome() == value.IsNone() {
			t.Fatalf("invalid state after decode: Some=%v None=%v", value.IsSome(), value.IsNone())
		}
		_, ok := value.Value()
		if ok != value.IsSome() {
			t.Fatalf("Value presence = %v, IsSome = %v", ok, value.IsSome())
		}
		if err != nil && value.OrZero() != "unchanged" {
			t.Fatalf("failed decode changed receiver to %#v", value.OrZero())
		}
	})
}

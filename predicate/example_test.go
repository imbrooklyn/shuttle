package predicate_test

import (
	"fmt"
	"strings"

	"github.com/imbrooklyn/shuttle/optional"
	"github.com/imbrooklyn/shuttle/predicate"
	"github.com/imbrooklyn/shuttle/stream"
)

func ExampleFunc() {
	nonBlank := predicate.Func[string](func(value string) bool {
		return strings.TrimSpace(value) != ""
	})

	optionalValue := optional.Some("Brooklyn").Filter(nonBlank)
	streamValues := stream.Of("Brooklyn", " ", "Shuttle").Filter(nonBlank).Collect()

	fmt.Println(optionalValue.Must())
	fmt.Println(streamValues)
	// Output:
	// Brooklyn
	// [Brooklyn Shuttle]
}

func ExampleFunc_Not() {
	isEven := predicate.Func[int](func(value int) bool { return value%2 == 0 })
	isOdd := isEven.Not()

	fmt.Println(isOdd(3), isOdd(4))
	// Output: true false
}

func ExampleFunc_And() {
	positive := predicate.Func[int](func(value int) bool { return value > 0 })
	even := predicate.Func[int](func(value int) bool { return value%2 == 0 })
	positiveEven := positive.And(even)

	fmt.Println(positiveEven(2), positiveEven(-2), positiveEven(3))
	// Output: true false false
}

func ExampleFunc_Or() {
	negative := predicate.Func[int](func(value int) bool { return value < 0 })
	zero := predicate.Equal(0)
	nonPositive := negative.Or(zero)

	fmt.Println(nonPositive(-1), nonPositive(0), nonPositive(1))
	// Output: true true false
}

func ExampleAlways() {
	keep := predicate.Always[string](true)
	drop := predicate.Always[string](false)

	fmt.Println(keep("any value"), drop("any value"))
	// Output: true false
}

func ExampleEqual() {
	isReady := predicate.Equal("ready")

	fmt.Println(isReady("ready"), isReady("waiting"))
	// Output: true false
}

func ExampleEqualFunc() {
	equalFold := predicate.EqualFunc("shuttle", strings.EqualFold)

	fmt.Println(equalFold("SHUTTLE"), equalFold("optional"))
	// Output: true false
}

func ExampleOn() {
	type user struct {
		name string
	}
	namedBrooklyn := predicate.On(func(value user) string { return value.name }, predicate.Equal("Brooklyn"))

	fmt.Println(namedBrooklyn(user{name: "Brooklyn"}), namedBrooklyn(user{name: "Queens"}))
	// Output: true false
}

func ExampleIsNil() {
	var pointer *int
	fmt.Println(predicate.IsNil(pointer), predicate.IsNil(0))
	// Output: true false
}

func ExampleIsNotNil() {
	value := 1
	fmt.Println(predicate.IsNotNil(&value), predicate.IsNotNil((*int)(nil)))
	// Output: true false
}

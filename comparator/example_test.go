package comparator_test

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/imbrooklyn/shuttle/comparator"
	"github.com/imbrooklyn/shuttle/stream"
)

func ExampleFunc() {
	compareLength := comparator.Func[string](func(left, right string) int {
		return cmp.Compare(len(left), len(right))
	})
	values := []string{"three", "a", "four"}
	slices.SortStableFunc(values, compareLength)
	fmt.Println(values)
	// Output: [a four three]
}

func ExampleOrdered() {
	compare := comparator.Ordered[int]()
	fmt.Println(compare(3, 5))
	fmt.Println(compare(5, 5))
	fmt.Println(compare(8, 5))
	// Output:
	// -1
	// 0
	// 1
}

func ExampleBy() {
	type item struct {
		name  string
		score int
	}
	values := []item{{"beta", 20}, {"alpha", 10}}
	slices.SortFunc(values, comparator.By(func(value item) int {
		return value.score
	}))
	fmt.Println(values)
	// Output: [{alpha 10} {beta 20}]
}

func ExampleByDescending() {
	values := []string{"beta", "alpha", "gamma"}
	slices.SortFunc(values, comparator.ByDescending(strings.ToLower))
	fmt.Println(values)
	// Output: [gamma beta alpha]
}

func ExampleOn() {
	type event struct {
		name string
		at   time.Time
	}
	compareTime := comparator.On(
		func(value event) time.Time { return value.at },
		time.Time.Compare,
	)
	earlier := event{"start", time.Unix(1, 0)}
	later := event{"finish", time.Unix(2, 0)}
	fmt.Println(compareTime(earlier, later))
	// Output: -1
}

func ExampleOnDescending() {
	type event struct {
		at time.Time
	}
	compareTime := comparator.OnDescending(
		func(value event) time.Time { return value.at },
		time.Time.Compare,
	)
	earlier := event{time.Unix(1, 0)}
	later := event{time.Unix(2, 0)}
	fmt.Println(compareTime(earlier, later))
	// Output: 1
}

func ExampleFunc_Reverse() {
	descending := comparator.Ordered[int]().Reverse()
	values := []int{2, 1, 3}
	slices.SortFunc(values, descending)
	fmt.Println(values)
	// Output: [3 2 1]
}

func ExampleFunc_Reverse_levels() {
	type item struct {
		rank int
		name string
	}
	values := []item{{1, "alpha"}, {2, "alpha"}, {1, "beta"}}
	ascending := comparator.By(func(value item) int {
		return value.rank
	}).ThenBy(func(value item) string {
		return value.name
	})

	wholeOrderDescending := slices.Clone(values)
	slices.SortStableFunc(wholeOrderDescending, ascending.Reverse())
	nameOnlyDescending := slices.Clone(values)
	slices.SortStableFunc(nameOnlyDescending, comparator.By(func(value item) int {
		return value.rank
	}).ThenByDescending(func(value item) string {
		return value.name
	}))

	fmt.Println(wholeOrderDescending)
	fmt.Println(nameOnlyDescending)
	// Output:
	// [{2 alpha} {1 beta} {1 alpha}]
	// [{1 beta} {1 alpha} {2 alpha}]
}

func ExampleFunc_interoperability() {
	type item struct {
		rank int
		name string
	}
	input := []item{{2, "beta"}, {1, "alpha"}, {1, "gamma"}}
	compare := comparator.By(func(value item) int {
		return value.rank
	}).ThenByDescending(func(value item) string {
		return value.name
	})

	standard := slices.Clone(input)
	slices.SortStableFunc(standard, compare)
	streamed := stream.FromSlice(input).SortedFunc(compare).Collect()

	fmt.Println(standard)
	fmt.Println(streamed)
	// Output:
	// [{1 gamma} {1 alpha} {2 beta}]
	// [{1 gamma} {1 alpha} {2 beta}]
}

func ExampleFunc_Then() {
	type item struct {
		score int
		name  string
	}
	compare := comparator.By(func(value item) int {
		return value.score
	}).Then(
		comparator.ByDescending(func(value item) string {
			return value.name
		}),
	)

	values := []item{{2, "beta"}, {1, "alpha"}, {1, "gamma"}}
	slices.SortStableFunc(values, compare)
	fmt.Println(values)
	// Output: [{1 gamma} {1 alpha} {2 beta}]
}

func ExampleFunc_ThenBy() {
	type item struct {
		score int
		name  string
	}
	compare := comparator.By(func(value item) int {
		return value.score
	}).ThenBy(func(value item) string {
		return value.name
	})

	values := []item{{2, "beta"}, {1, "gamma"}, {1, "alpha"}}
	slices.SortStableFunc(values, compare)
	fmt.Println(values)
	// Output: [{1 alpha} {1 gamma} {2 beta}]
}

func ExampleFunc_ThenByDescending() {
	type item struct {
		score int
		name  string
	}
	compare := comparator.By(func(value item) int {
		return value.score
	}).ThenByDescending(func(value item) string {
		return value.name
	})

	values := []item{{2, "beta"}, {1, "alpha"}, {1, "gamma"}}
	slices.SortStableFunc(values, compare)
	fmt.Println(values)
	// Output: [{1 gamma} {1 alpha} {2 beta}]
}

func ExampleFunc_ThenOn() {
	type event struct {
		category string
		at       time.Time
	}
	compare := comparator.By(func(value event) string {
		return value.category
	}).ThenOn(
		func(value event) time.Time { return value.at },
		time.Time.Compare,
	)

	left := event{"build", time.Unix(1, 0)}
	right := event{"build", time.Unix(2, 0)}
	fmt.Println(compare(left, right))
	// Output: -1
}

func ExampleFunc_ThenOnDescending() {
	type event struct {
		category string
		at       time.Time
	}
	compare := comparator.By(func(value event) string {
		return value.category
	}).ThenOnDescending(
		func(value event) time.Time { return value.at },
		time.Time.Compare,
	)

	left := event{"build", time.Unix(1, 0)}
	right := event{"build", time.Unix(2, 0)}
	fmt.Println(compare(left, right))
	// Output: 1
}

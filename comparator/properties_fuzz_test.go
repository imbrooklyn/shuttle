package comparator_test

import (
	"cmp"
	"math"
	"slices"
	"testing"

	"github.com/imbrooklyn/shuttle/comparator"
)

func FuzzReverseLaws(f *testing.F) {
	f.Add(1, 2)
	f.Add(2, 2)
	f.Add(math.MinInt, math.MaxInt)
	f.Fuzz(func(t *testing.T, left, right int) {
		base := comparator.Func[int](func(left, right int) int {
			switch {
			case left < right:
				return math.MinInt
			case left > right:
				return math.MaxInt
			default:
				return 0
			}
		})

		original := sign(base(left, right))
		if got := sign(base.Reverse()(left, right)); got != -original {
			t.Fatalf("Reverse sign = %d, want %d", got, -original)
		}
		if got := sign(base.Reverse().Reverse()(left, right)); got != original {
			t.Fatalf("double Reverse sign = %d, want %d", got, original)
		}
		if got := sign(comparator.OnDescending(func(value int) int { return value }, base)(left, right)); got != -original {
			t.Fatalf("OnDescending sign = %d, want %d", got, -original)
		}
		zero := comparator.Func[int](func(int, int) int { return 0 })
		if got := sign(zero.ThenOnDescending(func(value int) int { return value }, base)(left, right)); got != -original {
			t.Fatalf("ThenOnDescending sign = %d, want %d", got, -original)
		}
	})
}

type fuzzRecord struct {
	primary   int
	secondary int
	tertiary  int
}

func FuzzThenMatchesReference(f *testing.F) {
	f.Add(1, 2, 3, 1, 2, 4)
	f.Add(1, 5, 7, 1, 4, 8)
	f.Add(0, 0, 0, 0, 0, 0)
	f.Fuzz(func(t *testing.T, ap, as, at, bp, bs, bt int) {
		left := fuzzRecord{primary: ap, secondary: as, tertiary: at}
		right := fuzzRecord{primary: bp, secondary: bs, tertiary: bt}
		combined := comparator.By(func(value fuzzRecord) int { return value.primary }).
			ThenByDescending(func(value fuzzRecord) int { return value.secondary }).
			ThenOn(
				func(value fuzzRecord) int { return value.tertiary },
				comparator.Ordered[int](),
			)

		want := cmp.Compare(left.primary, right.primary)
		if want == 0 {
			want = cmp.Compare(right.secondary, left.secondary)
		}
		if want == 0 {
			want = cmp.Compare(left.tertiary, right.tertiary)
		}
		if got := sign(combined(left, right)); got != sign(want) {
			t.Fatalf("Then sign = %d, want %d", got, sign(want))
		}
	})
}

func FuzzFluentCompositionMatchesReference(f *testing.F) {
	f.Add(1, 2, 3, 1, 2, 4)
	f.Add(1, 5, 7, 1, 4, 8)
	f.Add(0, 0, 0, 0, 0, 0)
	f.Fuzz(func(t *testing.T, ap, as, at, bp, bs, bt int) {
		left := fuzzRecord{primary: ap, secondary: as, tertiary: at}
		right := fuzzRecord{primary: bp, secondary: bs, tertiary: bt}
		combined := comparator.By(func(value fuzzRecord) int { return value.primary }).
			ThenBy(func(value fuzzRecord) int { return value.secondary }).
			ThenOnDescending(
				func(value fuzzRecord) int { return value.tertiary },
				comparator.Ordered[int](),
			)

		want := cmp.Compare(left.primary, right.primary)
		if want == 0 {
			want = cmp.Compare(left.secondary, right.secondary)
		}
		if want == 0 {
			want = cmp.Compare(right.tertiary, left.tertiary)
		}
		if got := sign(combined(left, right)); got != sign(want) {
			t.Fatalf("fluent composition sign = %d, want %d", got, sign(want))
		}
	})
}

func FuzzByMatchesCompare(f *testing.F) {
	f.Add(-1, 1)
	f.Add(0, 0)
	f.Add(math.MinInt, math.MaxInt)
	f.Fuzz(func(t *testing.T, left, right int) {
		key := func(value int) int { return value }
		if got, want := comparator.By(key)(left, right), cmp.Compare(key(left), key(right)); got != want {
			t.Fatalf("By = %d, want %d", got, want)
		}
		if got, want := comparator.ByDescending(key)(left, right), cmp.Compare(key(right), key(left)); got != want {
			t.Fatalf("ByDescending = %d, want %d", got, want)
		}
	})
}

type stableRecord struct {
	key int
	id  int
}

func FuzzAscendingDescendingKeyOrder(f *testing.F) {
	f.Add([]byte{2, 1, 2, 0, 1})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64 {
			data = data[:64]
		}
		input := make([]stableRecord, len(data))
		for index, value := range data {
			input[index] = stableRecord{key: int(value % 8), id: index}
		}

		ascending := slices.Clone(input)
		descending := slices.Clone(input)
		slices.SortStableFunc(ascending, comparator.By(func(value stableRecord) int { return value.key }))
		slices.SortStableFunc(descending, comparator.ByDescending(func(value stableRecord) int { return value.key }))

		for index := range ascending {
			if got, want := descending[index].key, ascending[len(ascending)-1-index].key; got != want {
				t.Fatalf("descending key[%d] = %d, want %d", index, got, want)
			}
		}
		assertStableTies(t, ascending)
		assertStableTies(t, descending)
	})
}

func FuzzGeneratedOrderingLaws(f *testing.F) {
	f.Add(1, 2, 2, 1, 3, 0)
	f.Add(1, 1, 1, 1, 1, 1)
	f.Add(-1, 4, 0, 4, 1, 4)
	f.Fuzz(func(t *testing.T, ap, as, bp, bs, cp, cs int) {
		a := fuzzRecord{primary: ap, secondary: as}
		b := fuzzRecord{primary: bp, secondary: bs}
		c := fuzzRecord{primary: cp, secondary: cs}
		compare := comparator.By(func(value fuzzRecord) int { return value.primary }).Then(
			comparator.ByDescending(func(value fuzzRecord) int { return value.secondary }),
		)

		if got := compare(a, a); got != 0 {
			t.Fatalf("compare(a, a) = %d, want 0", got)
		}
		if got, want := sign(compare(a, b)), -sign(compare(b, a)); got != want {
			t.Fatalf("sign symmetry = %d, want %d", got, want)
		}
		ab := compare(a, b)
		bc := compare(b, c)
		ac := compare(a, c)
		if ab < 0 && bc < 0 && ac >= 0 {
			t.Fatalf("negative relation is not transitive: ab=%d bc=%d ac=%d", ab, bc, ac)
		}
		if ab == 0 && bc == 0 && ac != 0 {
			t.Fatalf("zero equivalence is not transitive: ab=%d bc=%d ac=%d", ab, bc, ac)
		}
	})
}

func assertStableTies(t *testing.T, values []stableRecord) {
	t.Helper()
	for index := 1; index < len(values); index++ {
		if values[index-1].key == values[index].key && values[index-1].id > values[index].id {
			t.Fatalf("unstable tie order at %d: %v", index, values)
		}
	}
}

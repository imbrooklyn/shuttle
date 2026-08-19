package comparator_test

import (
	"cmp"
	"slices"
	"testing"

	"github.com/imbrooklyn/shuttle/comparator"
	"github.com/imbrooklyn/shuttle/stream"
)

type benchmarkRecord struct {
	primary   int
	secondary string
	tertiary  int64
}

var (
	benchmarkIntSink           int
	benchmarkSliceSink         []benchmarkRecord
	benchmarkComparatorSink    comparator.Func[benchmarkRecord]
	benchmarkIntComparatorSink comparator.Func[int]
)

func BenchmarkOrdered(b *testing.B) {
	direct := cmp.Compare[int]
	composed := comparator.Ordered[int]()
	benchmarkEvaluation(b, direct, composed, 17, 42)
}

func BenchmarkBy(b *testing.B) {
	key := func(value benchmarkRecord) int { return value.primary }
	direct := func(left, right benchmarkRecord) int {
		leftKey := key(left)
		rightKey := key(right)
		return cmp.Compare(leftKey, rightKey)
	}
	composed := comparator.By(key)
	benchmarkEvaluation(b, direct, composed,
		benchmarkRecord{primary: 17}, benchmarkRecord{primary: 42})
}

func BenchmarkByDescending(b *testing.B) {
	key := func(value benchmarkRecord) int { return value.primary }
	direct := func(left, right benchmarkRecord) int {
		leftKey := key(left)
		rightKey := key(right)
		return cmp.Compare(rightKey, leftKey)
	}
	composed := comparator.ByDescending(key)
	benchmarkEvaluation(b, direct, composed,
		benchmarkRecord{primary: 17}, benchmarkRecord{primary: 42})
}

func BenchmarkOn(b *testing.B) {
	project := func(value benchmarkRecord) string { return value.secondary }
	compare := comparator.Func[string](cmp.Compare[string])
	direct := func(left, right benchmarkRecord) int {
		leftProjected := project(left)
		rightProjected := project(right)
		return compare(leftProjected, rightProjected)
	}
	composed := comparator.On(project, compare)
	benchmarkEvaluation(b, direct, composed,
		benchmarkRecord{secondary: "alpha"}, benchmarkRecord{secondary: "omega"})
}

func BenchmarkOnDescending(b *testing.B) {
	project := func(value benchmarkRecord) string { return value.secondary }
	compare := comparator.Func[string](cmp.Compare[string])
	direct := func(left, right benchmarkRecord) int {
		leftProjected := project(left)
		rightProjected := project(right)
		switch result := compare(leftProjected, rightProjected); {
		case result < 0:
			return 1
		case result > 0:
			return -1
		default:
			return 0
		}
	}
	composed := comparator.OnDescending(project, compare)
	benchmarkEvaluation(b, direct, composed,
		benchmarkRecord{secondary: "alpha"}, benchmarkRecord{secondary: "omega"})
}

func BenchmarkReverse(b *testing.B) {
	base := comparator.Func[int](func(left, right int) int { return left - right })
	direct := func(left, right int) int {
		switch result := base(left, right); {
		case result < 0:
			return 1
		case result > 0:
			return -1
		default:
			return 0
		}
	}
	composed := base.Reverse()
	benchmarkEvaluation(b, direct, composed, 17, 42)
}

func BenchmarkThen(b *testing.B) {
	primary := func(left, right benchmarkRecord) int {
		return cmp.Compare(left.primary, right.primary)
	}
	secondary := func(left, right benchmarkRecord) int {
		return cmp.Compare(right.secondary, left.secondary)
	}
	direct := func(left, right benchmarkRecord) int {
		if result := primary(left, right); result != 0 {
			return result
		}
		return secondary(left, right)
	}
	composed := comparator.Func[benchmarkRecord](primary).Then(secondary)
	benchmarkEvaluation(b, direct, composed,
		benchmarkRecord{primary: 1, secondary: "alpha"},
		benchmarkRecord{primary: 1, secondary: "omega"})
}

func BenchmarkThenBy(b *testing.B) {
	primary := comparator.Func[benchmarkRecord](func(left, right benchmarkRecord) int {
		return cmp.Compare(left.primary, right.primary)
	})
	key := func(value benchmarkRecord) string { return value.secondary }
	direct := func(left, right benchmarkRecord) int {
		if result := primary(left, right); result != 0 {
			return result
		}
		return cmp.Compare(key(left), key(right))
	}
	composed := primary.ThenBy(key)
	benchmarkEvaluation(b, direct, composed,
		benchmarkRecord{primary: 1, secondary: "alpha"},
		benchmarkRecord{primary: 1, secondary: "omega"})
}

func BenchmarkThenByDescending(b *testing.B) {
	primary := comparator.Func[benchmarkRecord](func(left, right benchmarkRecord) int {
		return cmp.Compare(left.primary, right.primary)
	})
	key := func(value benchmarkRecord) string { return value.secondary }
	direct := func(left, right benchmarkRecord) int {
		if result := primary(left, right); result != 0 {
			return result
		}
		leftKey := key(left)
		rightKey := key(right)
		return cmp.Compare(rightKey, leftKey)
	}
	composed := primary.ThenByDescending(key)
	benchmarkEvaluation(b, direct, composed,
		benchmarkRecord{primary: 1, secondary: "alpha"},
		benchmarkRecord{primary: 1, secondary: "omega"})
}

func BenchmarkThenOn(b *testing.B) {
	primary := comparator.Func[benchmarkRecord](func(left, right benchmarkRecord) int {
		return cmp.Compare(left.primary, right.primary)
	})
	project := func(value benchmarkRecord) int64 { return value.tertiary }
	compare := comparator.Func[int64](cmp.Compare[int64])
	direct := func(left, right benchmarkRecord) int {
		if result := primary(left, right); result != 0 {
			return result
		}
		return compare(project(left), project(right))
	}
	composed := primary.ThenOn(project, compare)
	benchmarkEvaluation(b, direct, composed,
		benchmarkRecord{primary: 1, tertiary: 17},
		benchmarkRecord{primary: 1, tertiary: 42})
}

func BenchmarkThenOnDescending(b *testing.B) {
	primary := comparator.Func[benchmarkRecord](func(left, right benchmarkRecord) int {
		return cmp.Compare(left.primary, right.primary)
	})
	project := func(value benchmarkRecord) int64 { return value.tertiary }
	compare := comparator.Func[int64](cmp.Compare[int64])
	direct := func(left, right benchmarkRecord) int {
		if result := primary(left, right); result != 0 {
			return result
		}
		switch result := compare(project(left), project(right)); {
		case result < 0:
			return 1
		case result > 0:
			return -1
		default:
			return 0
		}
	}
	composed := primary.ThenOnDescending(project, compare)
	benchmarkEvaluation(b, direct, composed,
		benchmarkRecord{primary: 1, tertiary: 17},
		benchmarkRecord{primary: 1, tertiary: 42})
}

func BenchmarkThreeLevelMixedOrdering(b *testing.B) {
	left := benchmarkRecord{primary: 1, secondary: "same", tertiary: 10}
	right := benchmarkRecord{primary: 1, secondary: "same", tertiary: 20}
	direct := func(left, right benchmarkRecord) int {
		if result := cmp.Compare(left.primary, right.primary); result != 0 {
			return result
		}
		if result := cmp.Compare(right.secondary, left.secondary); result != 0 {
			return result
		}
		return cmp.Compare(left.tertiary, right.tertiary)
	}
	composed := comparator.By(func(value benchmarkRecord) int { return value.primary }).
		ThenByDescending(func(value benchmarkRecord) string { return value.secondary }).
		ThenBy(func(value benchmarkRecord) int64 { return value.tertiary })
	benchmarkEvaluation(b, direct, composed, left, right)
}

func BenchmarkConstruction(b *testing.B) {
	b.Run("Ordered", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			benchmarkIntComparatorSink = comparator.Ordered[int]()
		}
	})
	b.Run("By", func(b *testing.B) {
		key := func(value benchmarkRecord) int { return value.primary }
		b.ReportAllocs()
		for range b.N {
			benchmarkComparatorSink = comparator.By(key)
		}
	})
	b.Run("ByDescending", func(b *testing.B) {
		key := func(value benchmarkRecord) string { return value.secondary }
		b.ReportAllocs()
		for range b.N {
			benchmarkComparatorSink = comparator.ByDescending(key)
		}
	})
	b.Run("On", func(b *testing.B) {
		project := func(value benchmarkRecord) int { return value.primary }
		compare := comparator.Ordered[int]()
		b.ReportAllocs()
		for range b.N {
			benchmarkComparatorSink = comparator.On(project, compare)
		}
	})
	b.Run("OnDescending", func(b *testing.B) {
		project := func(value benchmarkRecord) int { return value.primary }
		compare := comparator.Ordered[int]()
		b.ReportAllocs()
		for range b.N {
			benchmarkComparatorSink = comparator.OnDescending(project, compare)
		}
	})
	b.Run("Reverse", func(b *testing.B) {
		base := comparator.By(func(value benchmarkRecord) int { return value.primary })
		b.ReportAllocs()
		for range b.N {
			benchmarkComparatorSink = base.Reverse()
		}
	})
	b.Run("Then", func(b *testing.B) {
		primary := comparator.By(func(value benchmarkRecord) int { return value.primary })
		secondary := comparator.By(func(value benchmarkRecord) string { return value.secondary })
		b.ReportAllocs()
		for range b.N {
			benchmarkComparatorSink = primary.Then(secondary)
		}
	})
	b.Run("ThenBy", func(b *testing.B) {
		primary := comparator.By(func(value benchmarkRecord) int { return value.primary })
		key := func(value benchmarkRecord) string { return value.secondary }
		b.ReportAllocs()
		for range b.N {
			benchmarkComparatorSink = primary.ThenBy(key)
		}
	})
	b.Run("ThenByDescending", func(b *testing.B) {
		primary := comparator.By(func(value benchmarkRecord) int { return value.primary })
		key := func(value benchmarkRecord) string { return value.secondary }
		b.ReportAllocs()
		for range b.N {
			benchmarkComparatorSink = primary.ThenByDescending(key)
		}
	})
	b.Run("ThenOn", func(b *testing.B) {
		primary := comparator.By(func(value benchmarkRecord) int { return value.primary })
		project := func(value benchmarkRecord) int64 { return value.tertiary }
		compare := comparator.Ordered[int64]()
		b.ReportAllocs()
		for range b.N {
			benchmarkComparatorSink = primary.ThenOn(project, compare)
		}
	})
	b.Run("ThenOnDescending", func(b *testing.B) {
		primary := comparator.By(func(value benchmarkRecord) int { return value.primary })
		project := func(value benchmarkRecord) int64 { return value.tertiary }
		compare := comparator.Ordered[int64]()
		b.ReportAllocs()
		for range b.N {
			benchmarkComparatorSink = primary.ThenOnDescending(project, compare)
		}
	})
}

func BenchmarkSlicesSortStableFuncInteroperability(b *testing.B) {
	input := benchmarkInput(256)
	direct := func(left, right benchmarkRecord) int {
		if result := cmp.Compare(left.primary, right.primary); result != 0 {
			return result
		}
		return cmp.Compare(right.secondary, left.secondary)
	}
	composed := comparator.By(func(value benchmarkRecord) int { return value.primary }).
		ThenByDescending(func(value benchmarkRecord) string { return value.secondary })

	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			values := slices.Clone(input)
			slices.SortStableFunc(values, direct)
			benchmarkSliceSink = values
		}
	})
	b.Run("Comparator", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			values := slices.Clone(input)
			slices.SortStableFunc(values, composed)
			benchmarkSliceSink = values
		}
	})
}

func BenchmarkStreamSortedFuncInteroperability(b *testing.B) {
	input := benchmarkInput(256)
	direct := func(left, right benchmarkRecord) int {
		if result := cmp.Compare(left.primary, right.primary); result != 0 {
			return result
		}
		return cmp.Compare(right.secondary, left.secondary)
	}
	composed := comparator.By(func(value benchmarkRecord) int { return value.primary }).
		ThenByDescending(func(value benchmarkRecord) string { return value.secondary })

	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			benchmarkSliceSink = stream.FromSlice(input).SortedFunc(direct).Collect()
		}
	})
	b.Run("Comparator", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			benchmarkSliceSink = stream.FromSlice(input).SortedFunc(composed).Collect()
		}
	})
}

func benchmarkEvaluation[T any](
	b *testing.B,
	direct func(T, T) int,
	composed comparator.Func[T],
	left, right T,
) {
	b.Helper()
	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			benchmarkIntSink = direct(left, right)
		}
	})
	b.Run("Comparator", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			benchmarkIntSink = composed(left, right)
		}
	})
}

func benchmarkInput(size int) []benchmarkRecord {
	values := make([]benchmarkRecord, size)
	for index := range values {
		values[index] = benchmarkRecord{
			primary:   (index*37 + 11) % 23,
			secondary: string(rune('a' + (index*13+7)%26)),
			tertiary:  int64(index),
		}
	}
	return values
}

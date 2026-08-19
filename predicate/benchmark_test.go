package predicate_test

import (
	"testing"

	"github.com/imbrooklyn/shuttle/optional"
	"github.com/imbrooklyn/shuttle/predicate"
	"github.com/imbrooklyn/shuttle/stream"
)

var benchmarkBoolSink bool
var benchmarkIntSink int
var benchmarkOptionalSink optional.Optional[int]

func BenchmarkNot(b *testing.B) {
	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			benchmarkBoolSink = !(index%2 == 0)
		}
	})

	isEven := predicate.Func[int](func(value int) bool { return value%2 == 0 })
	isOdd := isEven.Not()
	b.Run("Predicate", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			benchmarkBoolSink = isOdd(index)
		}
	})
}

func BenchmarkAnd(b *testing.B) {
	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			benchmarkBoolSink = index >= 0 && index%2 == 0 && index < 1000
		}
	})

	composed := predicate.Func[int](func(value int) bool { return value >= 0 }).And(
		predicate.Func[int](func(value int) bool { return value%2 == 0 }),
		predicate.Func[int](func(value int) bool { return value < 1000 }),
	)
	b.Run("Predicate", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			benchmarkBoolSink = composed(index)
		}
	})
}

func BenchmarkOr(b *testing.B) {
	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			benchmarkBoolSink = index < 0 || index%2 == 0 || index == 1001
		}
	})

	composed := predicate.Func[int](func(value int) bool { return value < 0 }).Or(
		predicate.Func[int](func(value int) bool { return value%2 == 0 }),
		predicate.Equal(1001),
	)
	b.Run("Predicate", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			benchmarkBoolSink = composed(index)
		}
	})
}

type benchmarkRecord struct {
	score int
}

func BenchmarkOn(b *testing.B) {
	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			value := benchmarkRecord{score: index}
			benchmarkBoolSink = value.score%2 == 0
		}
	})

	projectedEven := predicate.On(
		func(value benchmarkRecord) int { return value.score },
		predicate.Func[int](func(value int) bool { return value%2 == 0 }),
	)
	b.Run("Predicate", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			benchmarkBoolSink = projectedEven(benchmarkRecord{score: index})
		}
	})
}

func BenchmarkIsNil(b *testing.B) {
	value := 1
	pointer := &value
	b.Run("DirectPointer", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			current := pointer
			if index&1 == 0 {
				current = nil
			}
			benchmarkBoolSink = current == nil
		}
	})
	b.Run("PredicatePointer", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			current := pointer
			if index&1 == 0 {
				current = nil
			}
			benchmarkBoolSink = predicate.IsNil(current)
		}
	})

	var typedNil any = (*int)(nil)
	b.Run("PredicateTypedNilInInterface", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			benchmarkBoolSink = predicate.IsNil(typedNil)
		}
	})
}

func BenchmarkOptionalFilterInteroperability(b *testing.B) {
	want := 42
	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			value := index & 63
			if value == want {
				benchmarkOptionalSink = optional.Some(value)
			} else {
				benchmarkOptionalSink = optional.None[int]()
			}
		}
	})

	equalWant := predicate.Equal(want)
	b.Run("Predicate", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			value := index & 63
			benchmarkOptionalSink = optional.Some(value).Filter(equalWant)
		}
	})
}

func BenchmarkStreamFilterInteroperability(b *testing.B) {
	values := make([]int, 1024)
	for index := range values {
		values[index] = index
	}

	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			sum := 0
			for _, value := range values {
				if value%2 == 0 {
					sum += value
				}
			}
			benchmarkIntSink = sum
		}
	})

	isEven := predicate.Func[int](func(value int) bool { return value%2 == 0 })
	filtered := stream.FromSlice(values).Filter(isEven)
	b.Run("Predicate", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			sum := 0
			filtered.ForEach(func(value int) { sum += value })
			benchmarkIntSink = sum
		}
	})
}

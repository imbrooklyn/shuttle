package stream_test

import (
	"cmp"
	"fmt"
	"slices"
	"testing"

	"github.com/imbrooklyn/shuttle/stream"
)

var (
	benchmarkIntSlice  []int
	benchmarkPairs     []stream.Pair[int, int]
	benchmarkIntSlices [][]int
)

var benchmarkSizes = []int{10, 1_000, 1_000_000}

var benchmarkBufferSizes = []int{8, 16, 64, 256, 257, 512, 4_096}

func benchmarkInput(size int) []int {
	values := make([]int, size)
	for index := range values {
		values[index] = index
	}
	return values
}

// Direct-loop destinations intentionally use the same nil-growth strategy as
// Collect. Input setup is outside the timed region for both implementations.
func BenchmarkFilterMapTake(b *testing.B) {
	for _, size := range benchmarkSizes {
		input := benchmarkInput(size)
		limit := size / 4
		b.Run(fmt.Sprintf("%d/Direct", size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				var result []int
				for _, value := range input {
					if value%2 != 0 {
						continue
					}
					result = append(result, value*2)
					if len(result) == limit {
						break
					}
				}
				benchmarkIntSlice = result
			}
		})
		b.Run(fmt.Sprintf("%d/Shuttle", size), func(b *testing.B) {
			pipeline := stream.FromSlice(input).
				Filter(func(value int) bool { return value%2 == 0 }).
				Map(func(value int) int { return value * 2 }).
				Take(limit)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkIntSlice = pipeline.Collect()
			}
		})
	}
}

func BenchmarkFlatMap(b *testing.B) {
	for _, size := range benchmarkSizes {
		input := benchmarkInput(size)
		b.Run(fmt.Sprintf("%d/Direct", size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				var result []int
				for _, value := range input {
					result = append(result, value, value)
				}
				benchmarkIntSlice = result
			}
		})
		b.Run(fmt.Sprintf("%d/Shuttle", size), func(b *testing.B) {
			pipeline := stream.FromSlice(input).FlatMap(func(value int) stream.Stream[int] {
				return stream.RepeatN(value, 2)
			})
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkIntSlice = pipeline.Collect()
			}
		})
	}
}

func benchmarkNestedInput(outerCount, innerCount int) [][]int {
	inner := benchmarkInput(innerCount)
	outer := make([][]int, outerCount)
	for index := range outer {
		outer[index] = inner
	}
	return outer
}

func benchmarkSliceFlatten(b *testing.B, input [][]int) {
	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			var result []int
			for _, values := range input {
				result = append(result, values...)
			}
			benchmarkIntSlice = result
		}
	})
	b.Run("FlatMapFromSlice", func(b *testing.B) {
		pipeline := stream.FromSlice(input).FlatMap(func(values []int) stream.Stream[int] {
			return stream.FromSlice(values)
		})
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			benchmarkIntSlice = pipeline.Collect()
		}
	})
	b.Run("FlatMapSlice", func(b *testing.B) {
		pipeline := stream.FromSlice(input).FlatMapSlice(func(values []int) []int { return values })
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			benchmarkIntSlice = pipeline.Collect()
		}
	})
}

func BenchmarkFlatMapSlice(b *testing.B) {
	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("ManySmall/%d", size), func(b *testing.B) {
			benchmarkSliceFlatten(b, benchmarkNestedInput(size, 2))
		})
		b.Run(fmt.Sprintf("OneLarge/%d", size), func(b *testing.B) {
			benchmarkSliceFlatten(b, benchmarkNestedInput(1, size))
		})
	}
}

func BenchmarkDistinctBy(b *testing.B) {
	for _, size := range benchmarkSizes {
		input := benchmarkInput(size)
		keySpace := size/4 + 1
		b.Run(fmt.Sprintf("%d/Direct", size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				seen := make(map[int]struct{})
				var result []int
				for _, value := range input {
					key := value % keySpace
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					result = append(result, value)
				}
				benchmarkIntSlice = result
			}
		})
		b.Run(fmt.Sprintf("%d/Shuttle", size), func(b *testing.B) {
			pipeline := stream.FromSlice(input).DistinctBy(func(value int) int { return value % keySpace })
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkIntSlice = pipeline.Collect()
			}
		})
	}
}

func BenchmarkZip(b *testing.B) {
	for _, size := range benchmarkSizes {
		left := benchmarkInput(size)
		right := benchmarkInput(size)
		b.Run(fmt.Sprintf("%d/Direct", size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				var result []stream.Pair[int, int]
				for index, value := range left {
					result = append(result, stream.Pair[int, int]{First: value, Second: right[index]})
				}
				benchmarkPairs = result
			}
		})
		b.Run(fmt.Sprintf("%d/Shuttle", size), func(b *testing.B) {
			pipeline := stream.Zip(stream.FromSlice(left), stream.FromSlice(right))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkPairs = pipeline.Collect()
			}
		})
	}
}

func BenchmarkChunk(b *testing.B) {
	const chunkSize = 16
	for _, size := range benchmarkSizes {
		input := benchmarkInput(size)
		b.Run(fmt.Sprintf("%d/Direct", size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				var result [][]int
				for start := 0; start < len(input); start += chunkSize {
					end := min(start+chunkSize, len(input))
					chunk := make([]int, end-start)
					copy(chunk, input[start:end])
					result = append(result, chunk)
				}
				benchmarkIntSlices = result
			}
		})
		b.Run(fmt.Sprintf("%d/Shuttle", size), func(b *testing.B) {
			pipeline := stream.Chunk(stream.FromSlice(input), chunkSize)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkIntSlices = pipeline.Collect()
			}
		})
	}
}

func BenchmarkWindow(b *testing.B) {
	const windowSize = 8
	for _, size := range benchmarkSizes {
		input := benchmarkInput(size)
		b.Run(fmt.Sprintf("%d/Direct", size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				var result [][]int
				for start := 0; start+windowSize <= len(input); start++ {
					window := make([]int, windowSize)
					copy(window, input[start:start+windowSize])
					result = append(result, window)
				}
				benchmarkIntSlices = result
			}
		})
		b.Run(fmt.Sprintf("%d/Shuttle", size), func(b *testing.B) {
			pipeline := stream.Window(stream.FromSlice(input), windowSize)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkIntSlices = pipeline.Collect()
			}
		})
	}
}

func BenchmarkStatefulBufferSizing(b *testing.B) {
	for _, size := range benchmarkBufferSizes {
		input := benchmarkInput(size * 4)
		b.Run(fmt.Sprintf("Chunk/%d", size), func(b *testing.B) {
			pipeline := stream.Chunk(stream.FromSlice(input), size)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkIntSlices = pipeline.Collect()
			}
		})
		b.Run(fmt.Sprintf("WindowStep/%d", size), func(b *testing.B) {
			pipeline := stream.WindowStep(stream.FromSlice(input), size, size)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkIntSlices = pipeline.Collect()
			}
		})
	}

	short := stream.Of(1, 2, 3, 4)
	b.Run("Chunk/HugeSizeShortInput", func(b *testing.B) {
		pipeline := stream.Chunk(short, int(^uint(0)>>1))
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			benchmarkIntSlices = pipeline.Collect()
		}
	})
	b.Run("WindowStep/HugeSizeShortInput", func(b *testing.B) {
		maxInt := int(^uint(0) >> 1)
		pipeline := stream.WindowStep(short, maxInt, maxInt)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			benchmarkIntSlices = pipeline.Collect()
		}
	})
}

func BenchmarkSortedFunc(b *testing.B) {
	for _, size := range benchmarkSizes {
		input := benchmarkInput(size)
		slices.Reverse(input)
		compare := cmp.Compare[int]
		b.Run(fmt.Sprintf("%d/Direct", size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				buffer := slices.Clone(input)
				slices.SortStableFunc(buffer, compare)
				benchmarkIntSlice = slices.Clone(buffer)
			}
		})
		b.Run(fmt.Sprintf("%d/Shuttle", size), func(b *testing.B) {
			pipeline := stream.FromSlice(input).SortedFunc(compare)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkIntSlice = pipeline.Collect()
			}
		})
	}
}

func BenchmarkCollect(b *testing.B) {
	for _, size := range benchmarkSizes {
		input := benchmarkInput(size)
		b.Run(fmt.Sprintf("%d/Direct", size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				var result []int
				//lint:ignore S1011 Keep the explicit direct loop as the benchmark baseline.
				for _, value := range input {
					result = append(result, value)
				}
				benchmarkIntSlice = result
			}
		})
		b.Run(fmt.Sprintf("%d/Shuttle", size), func(b *testing.B) {
			source := stream.FromSlice(input)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkIntSlice = source.Collect()
			}
		})
	}
}

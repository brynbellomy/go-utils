package iter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	biter "github.com/brynbellomy/go-utils/iter"
)

func TestMap(t *testing.T) {
	t.Run("transform integers", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5}
		seq := biter.SliceIterator(input)

		mapped := biter.Map(seq, func(x int) int { return x * 2 })

		var result []int
		for v := range mapped {
			result = append(result, v)
		}

		expected := []int{2, 4, 6, 8, 10}
		require.Equal(t, expected, result)
	})

	t.Run("transform to different type", func(t *testing.T) {
		input := []int{1, 2, 3}
		seq := biter.SliceIterator(input)

		mapped := biter.Map(seq, func(x int) string { return string(rune('a' + x - 1)) })

		var result []string
		for v := range mapped {
			result = append(result, v)
		}

		expected := []string{"a", "b", "c"}
		require.Equal(t, expected, result)
	})

	t.Run("empty sequence", func(t *testing.T) {
		var input []int
		seq := biter.SliceIterator(input)

		mapped := biter.Map(seq, func(x int) int { return x * 2 })

		var result []int
		for v := range mapped {
			result = append(result, v)
		}

		require.Empty(t, result)
	})

	t.Run("early termination", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5}
		seq := biter.SliceIterator(input)

		mapped := biter.Map(seq, func(x int) int { return x * 2 })

		var result []int
		for v := range mapped {
			result = append(result, v)
			if len(result) == 3 {
				break
			}
		}

		expected := []int{2, 4, 6}
		require.Equal(t, expected, result)
	})
}

func TestMap2(t *testing.T) {
	t.Run("transform key-value pairs", func(t *testing.T) {
		// Create a simple seq2 from a map
		m := map[string]int{"a": 1, "b": 2, "c": 3}
		seq := func(yield func(string, int) bool) {
			for k, v := range m {
				if !yield(k, v) {
					return
				}
			}
		}

		mapped := biter.Map2(seq, func(k string, v int) (string, int) {
			return k + k, v * 2
		})

		result := make(map[string]int)
		for k, v := range mapped {
			result[k] = v
		}

		expected := map[string]int{"aa": 2, "bb": 4, "cc": 6}
		require.Equal(t, expected, result)
	})

	t.Run("transform to different types", func(t *testing.T) {
		seq := func(yield func(int, string) bool) {
			pairs := []struct {
				k int
				v string
			}{{1, "one"}, {2, "two"}, {3, "three"}}
			for _, p := range pairs {
				if !yield(p.k, p.v) {
					return
				}
			}
		}

		mapped := biter.Map2(seq, func(k int, v string) (string, int) {
			return v, k * 10
		})

		result := make(map[string]int)
		for k, v := range mapped {
			result[k] = v
		}

		expected := map[string]int{"one": 10, "two": 20, "three": 30}
		require.Equal(t, expected, result)
	})

	t.Run("empty sequence", func(t *testing.T) {
		seq := func(yield func(string, int) bool) {
			// empty
		}

		mapped := biter.Map2(seq, func(k string, v int) (string, int) {
			return k, v
		})

		count := 0
		for range mapped {
			count++
		}

		require.Equal(t, 0, count)
	})
}

func TestMultiIterator(t *testing.T) {
	t.Run("concatenate multiple sequences", func(t *testing.T) {
		seq1 := biter.SliceIterator([]int{1, 2})
		seq2 := biter.SliceIterator([]int{3, 4})
		seq3 := biter.SliceIterator([]int{5, 6})

		multi := biter.MultiIterator(seq1, seq2, seq3)

		var result []int
		for v := range multi {
			result = append(result, v)
		}

		expected := []int{1, 2, 3, 4, 5, 6}
		require.Equal(t, expected, result)
	})

	t.Run("single sequence", func(t *testing.T) {
		seq := biter.SliceIterator([]int{1, 2, 3})
		multi := biter.MultiIterator(seq)

		var result []int
		for v := range multi {
			result = append(result, v)
		}

		expected := []int{1, 2, 3}
		require.Equal(t, expected, result)
	})

	t.Run("no sequences", func(t *testing.T) {
		multi := biter.MultiIterator[int]()

		var result []int
		for v := range multi {
			result = append(result, v)
		}

		require.Empty(t, result)
	})

	t.Run("some empty sequences", func(t *testing.T) {
		seq1 := biter.SliceIterator([]int{1, 2})
		seq2 := biter.SliceIterator([]int{})
		seq3 := biter.SliceIterator([]int{3, 4})

		multi := biter.MultiIterator(seq1, seq2, seq3)

		var result []int
		for v := range multi {
			result = append(result, v)
		}

		expected := []int{1, 2, 3, 4}
		require.Equal(t, expected, result)
	})

	t.Run("early termination", func(t *testing.T) {
		seq1 := biter.SliceIterator([]int{1, 2, 3})
		seq2 := biter.SliceIterator([]int{4, 5, 6})

		multi := biter.MultiIterator(seq1, seq2)

		var result []int
		for v := range multi {
			result = append(result, v)
			if len(result) == 4 {
				break
			}
		}

		expected := []int{1, 2, 3, 4}
		require.Equal(t, expected, result)
	})
}

func TestMultiIterator2(t *testing.T) {
	t.Run("concatenate key-value sequences", func(t *testing.T) {
		seq1 := func(yield func(string, int) bool) {
			pairs := []struct {
				k string
				v int
			}{{"a", 1}, {"b", 2}}
			for _, p := range pairs {
				if !yield(p.k, p.v) {
					return
				}
			}
		}

		seq2 := func(yield func(string, int) bool) {
			pairs := []struct {
				k string
				v int
			}{{"c", 3}, {"d", 4}}
			for _, p := range pairs {
				if !yield(p.k, p.v) {
					return
				}
			}
		}

		multi := biter.MultiIterator2(seq1, seq2)

		result := make(map[string]int)
		for k, v := range multi {
			result[k] = v
		}

		expected := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}
		require.Equal(t, expected, result)
	})

	t.Run("no sequences", func(t *testing.T) {
		multi := biter.MultiIterator2[string, int]()

		count := 0
		for range multi {
			count++
		}

		require.Equal(t, 0, count)
	})
}

func TestRangeIterator(t *testing.T) {
	t.Run("positive range", func(t *testing.T) {
		seq := biter.RangeIterator(1, 5)

		var result []int
		for v := range seq {
			result = append(result, v)
		}

		expected := []int{1, 2, 3, 4}
		require.Equal(t, expected, result)
	})

	t.Run("zero to positive", func(t *testing.T) {
		seq := biter.RangeIterator(0, 3)

		var result []int
		for v := range seq {
			result = append(result, v)
		}

		expected := []int{0, 1, 2}
		require.Equal(t, expected, result)
	})

	t.Run("negative range", func(t *testing.T) {
		seq := biter.RangeIterator(-3, 0)

		var result []int
		for v := range seq {
			result = append(result, v)
		}

		expected := []int{-3, -2, -1}
		require.Equal(t, expected, result)
	})

	t.Run("single element range", func(t *testing.T) {
		seq := biter.RangeIterator(5, 6)

		var result []int
		for v := range seq {
			result = append(result, v)
		}

		expected := []int{5}
		require.Equal(t, expected, result)
	})

	t.Run("empty range - equal start and end", func(t *testing.T) {
		seq := biter.RangeIterator(5, 5)

		var result []int
		for v := range seq {
			result = append(result, v)
		}

		require.Empty(t, result)
	})

	t.Run("empty range - start greater than end", func(t *testing.T) {
		seq := biter.RangeIterator(10, 5)

		var result []int
		for v := range seq {
			result = append(result, v)
		}

		require.Empty(t, result)
	})

	t.Run("early termination", func(t *testing.T) {
		seq := biter.RangeIterator(0, 10)

		var result []int
		for v := range seq {
			result = append(result, v)
			if len(result) == 3 {
				break
			}
		}

		expected := []int{0, 1, 2}
		require.Equal(t, expected, result)
	})

	t.Run("uint8 range", func(t *testing.T) {
		seq := biter.RangeIterator[uint8](250, 255)

		var result []uint8
		for v := range seq {
			result = append(result, v)
		}

		expected := []uint8{250, 251, 252, 253, 254}
		require.Equal(t, expected, result)
	})
}

func TestSliceIterator(t *testing.T) {
	t.Run("integer slice", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5}
		seq := biter.SliceIterator(input)

		var result []int
		for v := range seq {
			result = append(result, v)
		}

		require.Equal(t, input, result)
	})

	t.Run("string slice", func(t *testing.T) {
		input := []string{"hello", "world", "test"}
		seq := biter.SliceIterator(input)

		var result []string
		for v := range seq {
			result = append(result, v)
		}

		require.Equal(t, input, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		var input []int
		seq := biter.SliceIterator(input)

		var result []int
		for v := range seq {
			result = append(result, v)
		}

		require.Empty(t, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		var input []int
		seq := biter.SliceIterator(input)

		var result []int
		for v := range seq {
			result = append(result, v)
		}

		require.Empty(t, result)
	})

	t.Run("early termination", func(t *testing.T) {
		input := []int{10, 20, 30, 40, 50}
		seq := biter.SliceIterator(input)

		var result []int
		for v := range seq {
			result = append(result, v)
			if len(result) == 2 {
				break
			}
		}

		expected := []int{10, 20}
		require.Equal(t, expected, result)
	})

	t.Run("struct slice", func(t *testing.T) {
		type person struct {
			name string
			age  int
		}

		input := []person{
			{"Alice", 30},
			{"Bob", 25},
		}

		seq := biter.SliceIterator(input)

		var result []person
		for v := range seq {
			result = append(result, v)
		}

		require.Equal(t, input, result)
	})
}

func TestIntegration(t *testing.T) {
	t.Run("chain operations", func(t *testing.T) {
		// Create range 1-5, multiply by 2, then chain with another range
		range1 := biter.RangeIterator(1, 5)
		doubled := biter.Map(range1, func(x int) int { return x * 2 })

		range2 := biter.RangeIterator(10, 13)

		chained := biter.MultiIterator(doubled, range2)

		var result []int
		for v := range chained {
			result = append(result, v)
		}

		expected := []int{2, 4, 6, 8, 10, 11, 12}
		require.Equal(t, expected, result)
	})

	t.Run("slice to map transformation", func(t *testing.T) {
		input := []string{"a", "bb", "ccc"}
		seq := biter.SliceIterator(input)

		// Transform to (string, length) pairs
		mapped := biter.Map(seq, func(s string) struct {
			str string
			len int
		} {
			return struct {
				str string
				len int
			}{s, len(s)}
		})

		result := make(map[string]int)
		for v := range mapped {
			result[v.str] = v.len
		}

		expected := map[string]int{"a": 1, "bb": 2, "ccc": 3}
		require.Equal(t, expected, result)
	})
}

// Benchmark tests
func BenchmarkSliceIterator(b *testing.B) {
	slice := make([]int, 1000)
	for i := range slice {
		slice[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seq := biter.SliceIterator(slice)
		count := 0
		for range seq {
			count++
		}
	}
}

func BenchmarkRangeIterator(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seq := biter.RangeIterator(0, 1000)
		count := 0
		for range seq {
			count++
		}
	}
}

func BenchmarkMap(b *testing.B) {
	slice := make([]int, 1000)
	for i := range slice {
		slice[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seq := biter.SliceIterator(slice)
		mapped := biter.Map(seq, func(x int) int { return x * 2 })

		count := 0
		for range mapped {
			count++
		}
	}
}

// Complex chaining benchmarks to test inlining
func BenchmarkSimpleChain(b *testing.B) {
	slice := make([]int, 1000)
	for i := range slice {
		slice[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seq := biter.SliceIterator(slice)
		mapped := biter.Map(seq, func(x int) int { return x * 2 })
		mapped2 := biter.Map(mapped, func(x int) int { return x + 1 })

		sum := 0
		for v := range mapped2 {
			sum += v
		}
	}
}

func BenchmarkComplexChain(b *testing.B) {
	slice := make([]int, 1000)
	for i := range slice {
		slice[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seq := biter.SliceIterator(slice)
		mapped1 := biter.Map(seq, func(x int) int { return x * 2 })
		mapped2 := biter.Map(mapped1, func(x int) int { return x + 10 })
		mapped3 := biter.Map(mapped2, func(x int) int { return x * x })
		mapped4 := biter.Map(mapped3, func(x int) int { return x % 1000 })

		sum := 0
		for v := range mapped4 {
			sum += v
		}
	}
}

func BenchmarkMultiIteratorChain(b *testing.B) {
	slice1 := make([]int, 500)
	slice2 := make([]int, 500)
	for i := range slice1 {
		slice1[i] = i
		slice2[i] = i + 500
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seq1 := biter.SliceIterator(slice1)
		seq2 := biter.SliceIterator(slice2)
		multi := biter.MultiIterator(seq1, seq2)
		mapped := biter.Map(multi, func(x int) int { return x * 3 })

		sum := 0
		for v := range mapped {
			sum += v
		}
	}
}

func BenchmarkRangeToSliceChain(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		range1 := biter.RangeIterator(0, 500)
		range2 := biter.RangeIterator(500, 1000)
		multi := biter.MultiIterator(range1, range2)
		mapped := biter.Map(multi, func(x int) int { return x * x })

		sum := 0
		for v := range mapped {
			sum += v
		}
	}
}

// Compare against manual loops for baseline
func BenchmarkManualLoop(b *testing.B) {
	slice := make([]int, 1000)
	for i := range slice {
		slice[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sum := 0
		for _, x := range slice {
			v := x * 2
			v = v + 1
			sum += v
		}
	}
}

func BenchmarkManualComplexLoop(b *testing.B) {
	slice := make([]int, 1000)
	for i := range slice {
		slice[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sum := 0
		for _, x := range slice {
			v := x * 2
			v = v + 10
			v = v * v
			v = v % 1000
			sum += v
		}
	}
}

// Stress test with expensive operations
func BenchmarkExpensiveOperations(b *testing.B) {
	slice := make([]int, 100) // Smaller slice for expensive ops
	for i := range slice {
		slice[i] = i + 1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seq := biter.SliceIterator(slice)
		mapped := biter.Map(seq, func(x int) float64 {
			// Simulate expensive computation
			result := float64(x)
			for j := 0; j < 10; j++ {
				result = result * 1.1
			}
			return result
		})
		mapped2 := biter.Map(mapped, func(x float64) int {
			return int(x) % 1000
		})

		sum := 0
		for v := range mapped2 {
			sum += v
		}
	}
}

func BenchmarkTypeConversions(b *testing.B) {
	slice := make([]int, 1000)
	for i := range slice {
		slice[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seq := biter.SliceIterator(slice)
		mapped := biter.Map(seq, func(x int) float64 { return float64(x) * 2.5 })
		mapped2 := biter.Map(mapped, func(x float64) string { return "val" })
		mapped3 := biter.Map(mapped2, func(x string) int { return len(x) })

		sum := 0
		for v := range mapped3 {
			sum += v
		}
	}
}

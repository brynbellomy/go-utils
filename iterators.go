package utils

import (
	"cmp"
	"iter"

	"golang.org/x/exp/constraints"
)

func MultiIterator[X cmp.Ordered](iters ...iter.Seq[X]) iter.Seq[X] {
	return func(yield func(x X) bool) {
		for _, iter := range iters {
			for x := range iter {
				if !yield(x) {
					return
				}
			}
		}
	}
}

func MultiIterator2[K, V cmp.Ordered](iters ...iter.Seq2[K, V]) iter.Seq2[K, V] {
	return func(yield func(x K, v V) bool) {
		for _, iter := range iters {
			for k, v := range iter {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}

func RangeIterator[Elem constraints.Integer](start, end Elem) func(yield func(n Elem) bool) {
	return func(yield func(n Elem) bool) {
		for n := start; n < end; n++ {
			if !yield(n) {
				return
			}
		}
	}
}

func SliceIterator[T any](slice []T) func(yield func(t T) bool) {
	return func(yield func(t T) bool) {
		for _, t := range slice {
			if !yield(t) {
				return
			}
		}
	}
}

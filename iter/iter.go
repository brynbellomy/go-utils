package iter

import (
	"cmp"
	"iter"

	"golang.org/x/exp/constraints"
)

func Map[T, Out any](s iter.Seq[T], fn func(x T) Out) iter.Seq[Out] {
	return func(yield func(Out) bool) {
		for v := range s {
			if !yield(fn(v)) {
				return
			}
		}
	}
}

func Map2[T, U, Out1, Out2 any](seq iter.Seq2[T, U], fn func(t T, u U) (Out1, Out2)) iter.Seq2[Out1, Out2] {
	return func(yield func(Out1, Out2) bool) {
		for t, u := range seq {
			if !yield(fn(t, u)) {
				return
			}
		}
	}
}

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

func RangeIterator[Elem constraints.Integer](start, end Elem) iter.Seq[Elem] {
	return func(yield func(n Elem) bool) {
		for n := start; n < end; n++ {
			if !yield(n) {
				return
			}
		}
	}
}

func SliceIterator[T any](slice []T) iter.Seq[T] {
	return func(yield func(t T) bool) {
		for _, t := range slice {
			if !yield(t) {
				return
			}
		}
	}
}

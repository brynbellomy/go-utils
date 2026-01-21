package bcoll

import (
	"cmp"
	"fmt"
	"iter"
)

type SortedSet[K cmp.Ordered] KeySortedMap[K, struct{}]

func NewSortedSet[K cmp.Ordered]() *SortedSet[K] {
	return &SortedSet[K]{}
}

func (ss *SortedSet[K]) Clear() {
	(*KeySortedMap[K, struct{}])(ss).Clear()
}

func (ss *SortedSet[K]) Len() int {
	return (*KeySortedMap[K, struct{}])(ss).Len()
}

func (ss *SortedSet[K]) Insert(key K) {
	(*KeySortedMap[K, struct{}])(ss).Insert(key, struct{}{})
}

func (ss *SortedSet[K]) Has(key K) bool {
	_, ok := (*KeySortedMap[K, struct{}])(ss).Get(key)
	return ok
}

func (ss *SortedSet[K]) Iter() iter.Seq[K] {
	return func(yield func(k K) bool) {
		for k := range (*KeySortedMap[K, struct{}])(ss).Iter() {
			if !yield(k) {
				return
			}
		}
	}
}

func (ss *SortedSet[K]) ReverseIter() iter.Seq2[K, struct{}] {
	return (*KeySortedMap[K, struct{}])(ss).ReverseIter()
}

func (ss *SortedSet[K]) Slice() []K {
	return (*KeySortedMap[K, struct{}])(ss).Keys()
}

func (ss *SortedSet[K]) String() string {
	return fmt.Sprint(ss.Slice())
}

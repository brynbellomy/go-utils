package utils

import (
	"cmp"
	"fmt"
)

// SortedMap is a map that maintains keys in sorted order.

// SortedMap is a map that maintains keys in sorted order.
type SortedMap[K cmp.Ordered, V any] struct {
	root   *node[K, V]
	length int
}

type node[K cmp.Ordered, V any] struct {
	key   K
	value V
	left  *node[K, V]
	right *node[K, V]
}

func NewSortedMap[K cmp.Ordered, V any]() *SortedMap[K, V] {
	return &SortedMap[K, V]{}
}

func (sm *SortedMap[K, V]) Clear() {
	sm.root = nil
	sm.length = 0
}

func (sm *SortedMap[K, V]) Len() int {
	return sm.length
}

func (sm *SortedMap[K, V]) Insert(key K, value V) {
	sm.length++

	if sm.root == nil {
		sm.root = &node[K, V]{key: key, value: value}
		return
	}
	current := sm.root
	for {
		if key < current.key {
			if current.left == nil {
				current.left = &node[K, V]{key: key, value: value}
				return
			}
			current = current.left
		} else if key > current.key {
			if current.right == nil {
				current.right = &node[K, V]{key: key, value: value}
				return
			}
			current = current.right
		} else {
			// Key already exists, update the value.
			current.value = value
			return
		}
	}
}

func (sm *SortedMap[K, V]) Get(key K) (V, bool) {
	current := sm.root
	for current != nil {
		if key < current.key {
			current = current.left
		} else if key > current.key {
			current = current.right
		} else {
			return current.value, true
		}
	}
	var zero V
	return zero, false
}

func (sm *SortedMap[K, V]) Iter() func(yield func(k K, v V) bool) {
	return func(yield func(k K, v V) bool) {
		stack := []*node[K, V]{}
		current := sm.root

		// Continue until all nodes are processed.
		for current != nil || len(stack) > 0 {
			// Push all left nodes onto the stack.
			for current != nil {
				stack = append(stack, current)
				current = current.left
			}

			// Pop the top node.
			current = stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			// Yield the current key-value pair; stop if yield returns false.
			if !yield(current.key, current.value) {
				return
			}

			// Move to the right child.
			current = current.right
		}
	}
}

func (sm *SortedMap[K, V]) ReverseIter() func(yield func(k K, v V) bool) {
	return func(yield func(k K, v V) bool) {
		stack := []*node[K, V]{}
		current := sm.root

		// Continue until all nodes are processed.
		for current != nil || len(stack) > 0 {

			// Push all right nodes onto the stack.
			for current != nil {
				stack = append(stack, current)
				current = current.right
			}

			// Pop the top node.
			current = stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			// Yield the current key-value pair; stop if yield returns false.
			if !yield(current.key, current.value) {
				return
			}

			// Move to the left child.
			current = current.left
		}
	}
}

func (sm *SortedMap[K, V]) Keys() []K {
	xs := make([]K, sm.length)
	i := 0
	for x := range sm.Iter() {
		xs[i] = x
	}
	return xs
}

type SortedSet[K cmp.Ordered] SortedMap[K, struct{}]

func NewSortedSet[K cmp.Ordered]() *SortedSet[K] {
	return &SortedSet[K]{}
}

func (ss *SortedSet[K]) Clear() {
	(*SortedMap[K, struct{}])(ss).Clear()
}

func (ss *SortedSet[K]) Len() int {
	return (*SortedMap[K, struct{}])(ss).Len()
}

func (ss *SortedSet[K]) Insert(key K) {
	(*SortedMap[K, struct{}])(ss).Insert(key, struct{}{})
}

func (ss *SortedSet[K]) Has(key K) bool {
	_, ok := (*SortedMap[K, struct{}])(ss).Get(key)
	return ok
}

func (ss *SortedSet[K]) Iter() func(yield func(k K, v struct{}) bool) {
	return (*SortedMap[K, struct{}])(ss).Iter()
}

func (ss *SortedSet[K]) ReverseIter() func(yield func(k K, v struct{}) bool) {
	return (*SortedMap[K, struct{}])(ss).ReverseIter()
}

func (ss *SortedSet[K]) Slice() []K {
	return (*SortedMap[K, struct{}])(ss).Keys()
}

func (ss *SortedSet[K]) String() string {
	return fmt.Sprint(ss.Slice())
}

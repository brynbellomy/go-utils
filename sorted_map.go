package utils

import (
	"golang.org/x/exp/constraints"
)

// SortedMap is a map that maintains keys in sorted order.
type SortedMap[K constraints.Ordered, V any] struct {
	root *node[K, V]
}

type node[K constraints.Ordered, V any] struct {
	key   K
	value V
	left  *node[K, V]
	right *node[K, V]
}

func NewSortedMap[K constraints.Ordered, V any]() *SortedMap[K, V] {
	return &SortedMap[K, V]{}
}

func (sm *SortedMap[K, V]) Clear() {
	sm.root = nil
}

func (sm *SortedMap[K, V]) Len() int {
	count := 0
	for range sm.Iter() {
		count++
	}
	return count
}

func (sm *SortedMap[K, V]) Insert(key K, value V) {
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

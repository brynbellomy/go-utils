package bcoll

import (
	"fmt"
	"strings"
)

// InsertionOrderedSet represents a set that maintains insertion order
type InsertionOrderedSet[T comparable] struct {
	elements []T
	set      map[T]struct{}
}

func NewInsertionOrderedSet[T comparable]() *InsertionOrderedSet[T] {
	return &InsertionOrderedSet[T]{
		elements: []T{},
		set:      make(map[T]struct{}),
	}
}

// Add adds an element to the set if it doesn't already exist
func (s *InsertionOrderedSet[T]) Add(element T) {
	if _, exists := s.set[element]; !exists {
		s.elements = append(s.elements, element)
		s.set[element] = struct{}{}
	}
}

// Remove removes an element from the set
func (s *InsertionOrderedSet[T]) Remove(element T) {
	if _, exists := s.set[element]; exists {
		delete(s.set, element)
		for i, v := range s.elements {
			if v == element {
				s.elements = append(s.elements[:i], s.elements[i+1:]...)
				break
			}
		}
	}
}

// Contains checks if an element exists in the set
func (s *InsertionOrderedSet[T]) Contains(element T) bool {
	_, exists := s.set[element]
	return exists
}

// Size returns the number of elements in the set
func (s *InsertionOrderedSet[T]) Size() int {
	return len(s.elements)
}

// Clear removes all elements from the set
func (s *InsertionOrderedSet[T]) Clear() {
	s.elements = []T{}
	s.set = make(map[T]struct{})
}

// Elements returns a slice of all elements in the set, in order
func (s *InsertionOrderedSet[T]) Elements() []T {
	return append([]T{}, s.elements...)
}

// String returns a string representation of the set
func (s *InsertionOrderedSet[T]) String() string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, element := range s.elements {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%v", element))
	}
	sb.WriteString("]")
	return sb.String()
}

// Union returns a new InsertionOrderedSet containing all elements from both sets
func (s *InsertionOrderedSet[T]) Union(other *InsertionOrderedSet[T]) *InsertionOrderedSet[T] {
	result := NewInsertionOrderedSet[T]()
	for _, element := range s.elements {
		result.Add(element)
	}
	for _, element := range other.elements {
		result.Add(element)
	}
	return result
}

// Intersection returns a new InsertionOrderedSet containing elements common to both sets
func (s *InsertionOrderedSet[T]) Intersection(other *InsertionOrderedSet[T]) *InsertionOrderedSet[T] {
	result := NewInsertionOrderedSet[T]()
	for _, element := range s.elements {
		if other.Contains(element) {
			result.Add(element)
		}
	}
	return result
}

// Difference returns a new InsertionOrderedSet containing elements in s that are not in other
func (s *InsertionOrderedSet[T]) Difference(other *InsertionOrderedSet[T]) *InsertionOrderedSet[T] {
	result := NewInsertionOrderedSet[T]()
	for _, element := range s.elements {
		if !other.Contains(element) {
			result.Add(element)
		}
	}
	return result
}

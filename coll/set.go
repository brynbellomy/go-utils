package bcoll

import (
	"maps"
)

type Set[T comparable] map[T]struct{}

func NewSet[T comparable]() Set[T] {
	return Set[T](make(map[T]struct{}))
}

func NewSetFrom[T comparable](xs []T) Set[T] {
	s := Set[T](make(map[T]struct{}))
	s.AddSlice(xs)
	return s
}

func (m Set[T]) Has(item T) bool {
	_, ok := m[item]
	return ok
}

func (m Set[T]) Add(item T) (exists bool) {
	_, exists = m[item]
	m[item] = struct{}{}
	return exists
}

func (m Set[T]) AddSlice(items []T) {
	for _, item := range items {
		m.Add(item)
	}
}

func (m Set[T]) AddAll(items ...T) {
	for _, item := range items {
		m.Add(item)
	}
}

func (m Set[T]) AddSet(other Set[T]) {
	for item := range other {
		m.Add(item)
	}
}

func (m Set[T]) Remove(item T) bool {
	has := m.Has(item)
	delete(m, item)
	return has
}

func (m Set[T]) Copy() Set[T] {
	return Set[T](maps.Clone(m))
}

func (m Set[T]) Intersection(other Set[T]) Set[T] {
	result := NewSet[T]()
	for item := range m {
		if other.Has(item) {
			result.Add(item)
		}
	}
	return result
}

func (m Set[T]) Union(other Set[T]) Set[T] {
	result := m.Copy()
	for item := range other {
		result.Add(item)
	}
	return result
}

func (m Set[T]) Difference(other Set[T]) Set[T] {
	result := NewSet[T]()
	for item := range m {
		if !other.Has(item) {
			result.Add(item)
		}
	}
	return result
}

func (m Set[T]) Slice() []T {
	out := make([]T, 0, len(m))
	for item := range m {
		out = append(out, item)
	}
	return out
}

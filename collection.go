package utils

type Set[T comparable] map[T]struct{}

func NewSet[T comparable]() Set[T] {
	return Set[T](make(map[T]struct{}))
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

func (m Set[T]) AddAll(items ...T) {
	for _, item := range items {
		m.Add(item)
	}
}

func (m Set[T]) Remove(item T) bool {
	has := m.Has(item)
	delete(m, item)
	return has
}

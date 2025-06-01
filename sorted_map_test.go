package utils

import (
	"reflect"
	"testing"
)

func TestSortedMapInsertAndGet(t *testing.T) {
	// Create a new SortedMap with string values
	sm := NewSortedMap[int, string]()

	// Insert key-value pairs
	sm.Insert(3, "three")
	sm.Insert(1, "one")
	sm.Insert(4, "four")
	sm.Insert(-1, "negative one")

	// Define test cases
	tests := []struct {
		key      int
		expected string
		exists   bool
	}{
		{1, "one", true},           // Existing key
		{3, "three", true},         // Existing key
		{4, "four", true},          // Existing key
		{-1, "negative one", true}, // Negative key
		{2, "", false},             // Non-existing key
		{0, "", false},             // Non-existing key
	}

	// Run each test case
	for _, test := range tests {
		val, ok := sm.Get(test.key)
		if ok != test.exists {
			t.Errorf("For key %d, expected exists=%v, got %v", test.key, test.exists, ok)
		}
		if ok && val != test.expected {
			t.Errorf("For key %d, expected value %q, got %q", test.key, test.expected, val)
		}
	}
}

func TestSortedMapIterator(t *testing.T) {
	// Create a new SortedMap with string values
	sm := NewSortedMap[int, string]()

	// Step 1: Test iteration on an empty map
	var gotKeys []int
	var gotValues []string
	for k, v := range sm.Iter() {
		gotKeys = append(gotKeys, k)
		gotValues = append(gotValues, v)
	}
	if len(gotKeys) != 0 {
		t.Errorf("Expected no keys on empty map, got %v", gotKeys)
	}
	if len(gotValues) != 0 {
		t.Errorf("Expected no values on empty map, got %v", gotValues)
	}

	// Step 2: Insert initial elements and test iteration
	sm.Insert(-5, "neg five")
	sm.Insert(0, "zero")
	sm.Insert(5, "five")
	sm.Insert(10, "ten")

	// Collect keys and values using the iterator
	gotKeys = nil // Reset slices
	gotValues = nil
	for k, v := range sm.Iter() {
		gotKeys = append(gotKeys, k)
		gotValues = append(gotValues, v)
	}

	// Define expected keys and values (keys should be in sorted order)
	expectedKeys := []int{-5, 0, 5, 10}
	expectedValues := []string{"neg five", "zero", "five", "ten"}

	// Verify the results
	if !reflect.DeepEqual(gotKeys, expectedKeys) {
		t.Errorf("After initial insert, expected keys %v, got %v", expectedKeys, gotKeys)
	}
	if !reflect.DeepEqual(gotValues, expectedValues) {
		t.Errorf("After initial insert, expected values %v, got %v", expectedValues, gotValues)
	}

	// Step 3: Add more elements (prepend, append, and insert in between)
	sm.Insert(-10, "neg ten") // Prepends (less than -5)
	sm.Insert(-2, "neg two")  // Between -5 and 0
	sm.Insert(2, "two")       // Between 0 and 5
	sm.Insert(7, "seven")     // Between 5 and 10
	sm.Insert(15, "fifteen")  // Appends (greater than 10)

	// Collect keys and values again
	gotKeys = nil // Reset slices
	gotValues = nil
	for k, v := range sm.Iter() {
		gotKeys = append(gotKeys, k)
		gotValues = append(gotValues, v)
	}

	// Define expected keys and values after additional insertions
	expectedKeys = []int{-10, -5, -2, 0, 2, 5, 7, 10, 15}
	expectedValues = []string{"neg ten", "neg five", "neg two", "zero", "two", "five", "seven", "ten", "fifteen"}

	// Verify the results
	if !reflect.DeepEqual(gotKeys, expectedKeys) {
		t.Errorf("After additional inserts, expected keys %v, got %v", expectedKeys, gotKeys)
	}
	if !reflect.DeepEqual(gotValues, expectedValues) {
		t.Errorf("After additional inserts, expected values %v, got %v", expectedValues, gotValues)
	}
}

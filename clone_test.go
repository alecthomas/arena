package arena

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

type NestedStruct struct {
	X int
	Y string
}

type TestStruct struct {
	Int      int
	IntPtr   *int
	Ptr      *NestedStruct
	Slice    []int
	Map      map[string]*NestedStruct
	Nested   NestedStruct
	PtrSlice []*NestedStruct
	NilPtr   *NestedStruct
	NilSlice []int
	NilMap   map[string]*NestedStruct
	Array    [2]int
}

func p[T any](v T) *T { return &v }

func TestClone(t *testing.T) {
	// Create test data
	original := TestStruct{
		Int:    42,
		IntPtr: p(42),
		Ptr: &NestedStruct{
			X: 1,
			Y: "hello",
		},
		Slice: []int{1, 2, 3},
		Map: map[string]*NestedStruct{
			"key1": {X: 10, Y: "world"},
			"key2": {X: 20, Y: "test"},
		},
		Nested: NestedStruct{
			X: 5,
			Y: "nested",
		},
		PtrSlice: []*NestedStruct{
			{X: 30, Y: "slice1"},
			{X: 40, Y: "slice2"},
		},
		NilPtr:   nil,
		NilSlice: nil,
		NilMap:   nil,
		Array:    [2]int{1, 2},
	}

	// Create arena and clone
	a := Create(1024)
	cloned := Clone(a, original)

	// Verify the clone is not nil and has the same content
	assert.Equal(t, original, *cloned)

	// Verify pointer fields are different objects
	if original.Ptr == cloned.Ptr {
		t.Error("Ptr should be a different object")
	}

	// Verify map values are different objects
	for k, v := range original.Map {
		if v == cloned.Map[k] {
			t.Errorf("Map value for key %s should be a different object", k)
		}
	}

	// Verify slice elements are different objects
	for i, v := range original.PtrSlice {
		if v == cloned.PtrSlice[i] {
			t.Errorf("PtrSlice[%d] should be a different object", i)
		}
	}

	// Verify arena allocation
	if a.cursor.Load() == 0 {
		t.Error("Arena should have allocated memory")
	}
}

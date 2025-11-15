package vtc

import (
	"testing"
)

func TestMacroStore(t *testing.T) {
	ms := NewMacroStore()

	// Test Define and Get
	ms.Define("foo", "bar")
	val, ok := ms.Get("foo")
	if !ok {
		t.Error("Expected macro 'foo' to be defined")
	}
	if val != "bar" {
		t.Errorf("Expected value 'bar', got '%s'", val)
	}

	// Test Definef
	ms.Definef("num", "%d", 42)
	val, ok = ms.Get("num")
	if !ok {
		t.Error("Expected macro 'num' to be defined")
	}
	if val != "42" {
		t.Errorf("Expected value '42', got '%s'", val)
	}

	// Test Exists
	if !ms.Exists("foo") {
		t.Error("Expected 'foo' to exist")
	}
	if ms.Exists("notdefined") {
		t.Error("Expected 'notdefined' to not exist")
	}

	// Test Count
	if ms.Count() != 2 {
		t.Errorf("Expected 2 macros, got %d", ms.Count())
	}

	// Test Delete
	ms.Delete("foo")
	if ms.Exists("foo") {
		t.Error("Expected 'foo' to be deleted")
	}
}

func TestMacroExpansion(t *testing.T) {
	ms := NewMacroStore()
	ms.Define("name", "world")
	ms.Define("count", "42")

	tests := []struct {
		input    string
		expected string
		hasError bool
	}{
		{"hello ${name}", "hello world", false},
		{"count is ${count}", "count is 42", false},
		{"${name} ${count}", "world 42", false},
		{"no macros here", "no macros here", false},
		{"${name}${count}", "world42", false},
		{"${undefined}", "", true},
		{"text ${name} more ${count} text", "text world more 42 text", false},
	}

	for _, tt := range tests {
		result, err := ms.Expand(nil, tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("Expected error for input %q", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("Unexpected error for input %q: %v", tt.input, err)
			continue
		}
		if result != tt.expected {
			t.Errorf("For input %q, expected %q, got %q", tt.input, tt.expected, result)
		}
	}
}

func TestMacroClone(t *testing.T) {
	ms1 := NewMacroStore()
	ms1.Define("foo", "bar")
	ms1.Define("num", "42")

	ms2 := ms1.Clone()

	// Verify clone has the same macros
	if ms2.Count() != ms1.Count() {
		t.Errorf("Clone has different count: expected %d, got %d", ms1.Count(), ms2.Count())
	}

	val, ok := ms2.Get("foo")
	if !ok || val != "bar" {
		t.Error("Clone doesn't have 'foo' macro")
	}

	// Modify clone
	ms2.Define("baz", "qux")

	// Original should not be affected
	if ms1.Exists("baz") {
		t.Error("Original was affected by clone modification")
	}
}

func TestMacroMerge(t *testing.T) {
	ms1 := NewMacroStore()
	ms1.Define("foo", "bar")

	ms2 := NewMacroStore()
	ms2.Define("baz", "qux")
	ms2.Define("num", "42")

	ms1.Merge(ms2)

	if ms1.Count() != 3 {
		t.Errorf("Expected 3 macros after merge, got %d", ms1.Count())
	}

	if !ms1.Exists("baz") {
		t.Error("Merged macro 'baz' not found")
	}
}

func TestMacroClear(t *testing.T) {
	ms := NewMacroStore()
	ms.Define("foo", "bar")
	ms.Define("baz", "qux")

	ms.Clear()

	if ms.Count() != 0 {
		t.Errorf("Expected 0 macros after clear, got %d", ms.Count())
	}
}

func TestMacroDefineMultiple(t *testing.T) {
	ms := NewMacroStore()
	macros := map[string]string{
		"foo": "bar",
		"baz": "qux",
		"num": "42",
	}

	ms.DefineMultiple(macros)

	if ms.Count() != 3 {
		t.Errorf("Expected 3 macros, got %d", ms.Count())
	}

	for k, v := range macros {
		val, ok := ms.Get(k)
		if !ok {
			t.Errorf("Macro %s not found", k)
		}
		if val != v {
			t.Errorf("Macro %s has wrong value: expected %s, got %s", k, v, val)
		}
	}
}

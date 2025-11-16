package util

import (
	"testing"
)

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
		hasError bool
	}{
		{"one two three", []string{"one", "two", "three"}, false},
		{`"one" "two" "three"`, []string{"one", "two", "three"}, false},
		{`one "two three" four`, []string{"one", "two three", "four"}, false},
		{`one \"quoted\" word`, []string{"one", `"quoted"`, "word"}, false},
		{`"unterminated`, nil, true},
		{`trailing\`, nil, true},
	}

	for _, tt := range tests {
		result, err := SplitArgs(tt.input)
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
		if len(result) != len(tt.expected) {
			t.Errorf("For input %q, expected %d args, got %d", tt.input, len(tt.expected), len(result))
			continue
		}
		for i, exp := range tt.expected {
			if result[i] != exp {
				t.Errorf("For input %q, arg %d: expected %q, got %q", tt.input, i, exp, result[i])
			}
		}
	}
}

func TestUnquoteString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`{hello}`, "hello"},
		{`hello`, "hello"},
		{`  "hello"  `, "hello"},
		{`{multi line}`, "multi line"},
	}

	for _, tt := range tests {
		result, err := UnquoteString(tt.input)
		if err != nil {
			t.Errorf("Unexpected error for input %q: %v", tt.input, err)
			continue
		}
		if result != tt.expected {
			t.Errorf("For input %q, expected %q, got %q", tt.input, tt.expected, result)
		}
	}
}

func TestGenerateBody(t *testing.T) {
	body := GenerateBody(100, "X")
	if len(body) != 100 {
		t.Errorf("Expected length 100, got %d", len(body))
	}

	body = GenerateBody(10, "ABC")
	if len(body) != 10 {
		t.Errorf("Expected length 10, got %d", len(body))
	}
	expected := "ABCABCABCA"
	if body != expected {
		t.Errorf("Expected %q, got %q", expected, body)
	}
}

func TestStripComments(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"no comment", "no comment"},
		{"text # comment", "text "},
		{"# full comment", ""},
		{"text#noSpace", "text"},
		// Test quoted strings with # inside
		{`process p7 -write "#"`, `process p7 -write "#"`},
		{`process p7 -write "\x1b(A#$%\x1b)A"`, `process p7 -write "\x1b(A#$%\x1b)A"`},
		{`vtest "Test with # in title"`, `vtest "Test with # in title"`},
		{`text "quoted # string" # comment`, `text "quoted # string" `},
		// Test escaped quotes
		{`text "quote with \" escape" # comment`, `text "quote with \" escape" `},
		{`text "quote with \\" # comment`, `text "quote with \\" `},
	}

	for _, tt := range tests {
		result := StripComments(tt.input)
		if result != tt.expected {
			t.Errorf("For input %q, expected %q, got %q", tt.input, tt.expected, result)
		}
	}
}

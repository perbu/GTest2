// Package util provides string manipulation and parsing utilities
package util

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// IsSpace checks if a character is whitespace
func IsSpace(c rune) bool {
	return unicode.IsSpace(c)
}

// IsDigit checks if a character is a digit
func IsDigit(c rune) bool {
	return c >= '0' && c <= '9'
}

// IsAlpha checks if a character is alphabetic
func IsAlpha(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// IsAlnum checks if a character is alphanumeric
func IsAlnum(c rune) bool {
	return IsAlpha(c) || IsDigit(c)
}

// TrimSpace removes leading and trailing whitespace
func TrimSpace(s string) string {
	return strings.TrimSpace(s)
}

// SplitArgs parses a command-line like string into arguments
// Handles quoted strings and backslash escapes
func SplitArgs(s string) ([]string, error) {
	var args []string
	var current strings.Builder
	inQuote := false
	escape := false

	for i, c := range s {
		if escape {
			// Handle escape sequences
			switch c {
			case 'n':
				current.WriteRune('\n')
			case 'r':
				current.WriteRune('\r')
			case 't':
				current.WriteRune('\t')
			case '\\':
				current.WriteRune('\\')
			case '"':
				current.WriteRune('"')
			case 'x':
				// Hex escape: \xNN
				if i+2 < len(s) {
					hex := s[i+1 : i+3]
					val, err := strconv.ParseInt(hex, 16, 32)
					if err == nil {
						current.WriteRune(rune(val))
					}
				}
			default:
				current.WriteRune(c)
			}
			escape = false
			continue
		}

		if c == '\\' {
			escape = true
			continue
		}

		if c == '"' {
			inQuote = !inQuote
			continue
		}

		if !inQuote && IsSpace(c) {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(c)
	}

	if escape {
		return nil, fmt.Errorf("trailing backslash")
	}

	if inQuote {
		return nil, fmt.Errorf("unterminated quote")
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args, nil
}

// UnquoteString handles both double-quoted and curly-braced strings
func UnquoteString(s string) (string, error) {
	s = strings.TrimSpace(s)

	// Handle curly-braced strings (VTC-style)
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		return s[1 : len(s)-1], nil
	}

	// Handle double-quoted strings
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		// Use strconv.Unquote for proper escape handling
		return strconv.Unquote(s)
	}

	// Return as-is if not quoted
	return s, nil
}

// ParseInt parses an integer from a string
func ParseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// ParseFloat parses a float from a string
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// Contains checks if a string contains a substring
func Contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// HasPrefix checks if a string has a prefix
func HasPrefix(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

// HasSuffix checks if a string has a suffix
func HasSuffix(s, suffix string) bool {
	return strings.HasSuffix(s, suffix)
}

// Join joins strings with a separator
func Join(elems []string, sep string) string {
	return strings.Join(elems, sep)
}

// Split splits a string by a separator
func Split(s, sep string) []string {
	return strings.Split(s, sep)
}

// IndexByte finds the first occurrence of a byte
func IndexByte(s string, c byte) int {
	return strings.IndexByte(s, c)
}

// ToLower converts a string to lowercase
func ToLower(s string) string {
	return strings.ToLower(s)
}

// ToUpper converts a string to uppercase
func ToUpper(s string) string {
	return strings.ToUpper(s)
}

// EqualFold compares strings case-insensitively
func EqualFold(s1, s2 string) bool {
	return strings.EqualFold(s1, s2)
}

// GenerateBody generates a body string of the specified length
// Used for -bodylen in HTTP commands
func GenerateBody(length int, pattern string) string {
	if pattern == "" {
		pattern = "X"
	}

	var buf strings.Builder
	buf.Grow(length)

	for buf.Len() < length {
		remaining := length - buf.Len()
		if remaining >= len(pattern) {
			buf.WriteString(pattern)
		} else {
			buf.WriteString(pattern[:remaining])
		}
	}

	return buf.String()
}

// Lines splits a string into lines
func Lines(s string) []string {
	return strings.Split(s, "\n")
}

// StripComments removes # comments from a line
func StripComments(line string) string {
	if idx := strings.IndexByte(line, '#'); idx >= 0 {
		return line[:idx]
	}
	return line
}

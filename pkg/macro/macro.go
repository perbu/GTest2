// Package macro provides macro management for VTC
package macro

import (
	"fmt"
	"strings"
	"sync"

	"github.com/perbu/GTest/pkg/logging"
)

// Store manages macro definitions and expansion
type Store struct {
	macros map[string]string
	mutex  sync.RWMutex
}

// New creates a new macro store
func New() *Store {
	return &Store{
		macros: make(map[string]string),
	}
}

// Define defines a macro with a name and value
func (ms *Store) Define(name, value string) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	ms.macros[name] = value
}

// Definef defines a macro with a formatted value
func (ms *Store) Definef(name, format string, args ...interface{}) {
	ms.Define(name, fmt.Sprintf(format, args...))
}

// Get retrieves a macro value
func (ms *Store) Get(name string) (string, bool) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	val, ok := ms.macros[name]
	return val, ok
}

// Delete removes a macro definition
func (ms *Store) Delete(name string) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	delete(ms.macros, name)
}

// All returns all macro definitions
func (ms *Store) All() map[string]string {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	result := make(map[string]string, len(ms.macros))
	for k, v := range ms.macros {
		result[k] = v
	}
	return result
}

// Expand expands all ${name} macros in the text
func (ms *Store) Expand(logger *logging.Logger, text string) (string, error) {
	var result strings.Builder
	result.Grow(len(text))

	for len(text) > 0 {
		// Find the next macro reference
		start := strings.Index(text, "${")
		if start == -1 {
			// No more macros, append remaining text
			result.WriteString(text)
			break
		}

		// Append text before the macro
		result.WriteString(text[:start])

		// Find the end of the macro reference
		end := strings.Index(text[start:], "}")
		if end == -1 {
			// No closing brace, append remaining text as-is
			result.WriteString(text[start:])
			break
		}

		// Extract macro name
		end += start // Convert to absolute position
		macroName := text[start+2 : end]

		// Look up macro value
		value, ok := ms.Get(macroName)
		if !ok {
			// Try dynamic macro expansion (e.g., functions)
			value, ok = ms.expandDynamic(logger, macroName)
			if !ok {
				if logger != nil {
					logger.Error("Macro ${%s} not found", macroName)
				}
				return "", fmt.Errorf("macro ${%s} not found", macroName)
			}
		}

		// Append macro value
		result.WriteString(value)

		// Move past the macro
		text = text[end+1:]
	}

	return result.String(), nil
}

// expandDynamic handles dynamic macro expansion (functions, etc.)
func (ms *Store) expandDynamic(logger *logging.Logger, name string) (string, bool) {
	// For now, we don't support dynamic macros
	// In the future, this could handle things like:
	// - ${rand} for random numbers
	// - ${date} for current date
	// - Function calls with arguments
	return "", false
}

// Clone creates a copy of the macro store
func (ms *Store) Clone() *Store {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	clone := New()
	for k, v := range ms.macros {
		clone.macros[k] = v
	}
	return clone
}

// Merge merges another macro store into this one
func (ms *Store) Merge(other *Store) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	other.mutex.RLock()
	defer other.mutex.RUnlock()

	for k, v := range other.macros {
		ms.macros[k] = v
	}
}

// MustExpand expands macros and panics on error
func (ms *Store) MustExpand(logger *logging.Logger, text string) string {
	result, err := ms.Expand(logger, text)
	if err != nil {
		panic(err)
	}
	return result
}

// ExpandOrDefault expands macros, returning defaultValue on error
func (ms *Store) ExpandOrDefault(logger *logging.Logger, text, defaultValue string) string {
	result, err := ms.Expand(logger, text)
	if err != nil {
		return defaultValue
	}
	return result
}

// Count returns the number of defined macros
func (ms *Store) Count() int {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	return len(ms.macros)
}

// Clear removes all macro definitions
func (ms *Store) Clear() {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	ms.macros = make(map[string]string)
}

// Exists checks if a macro is defined
func (ms *Store) Exists(name string) bool {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	_, ok := ms.macros[name]
	return ok
}

// DefineMultiple defines multiple macros at once
func (ms *Store) DefineMultiple(macros map[string]string) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	for k, v := range macros {
		ms.macros[k] = v
	}
}

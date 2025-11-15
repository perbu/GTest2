// Package vtc provides VTC (Varnish Test Case) language parsing and execution
package vtc

import (
	"github.com/perbu/gvtest/pkg/macro"
)

// MacroStore is an alias for macro.Store for backward compatibility
type MacroStore = macro.Store

// NewMacroStore creates a new macro store
func NewMacroStore() *MacroStore {
	return macro.New()
}

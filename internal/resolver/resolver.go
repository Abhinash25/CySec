// Package resolver implements the tool resolution pipeline:
// command → built-in check → installed check → registry lookup → unknown handler.
package resolver

import (
	"github.com/cysec-env/cysec/internal/database"
	"github.com/cysec-env/cysec/internal/registry"
)

// Resolution represents the outcome of resolving a command.
type Resolution struct {
	Status       Status
	Tool         *registry.ToolEntry // Set when found in registry
	InstalledTool *database.InstalledTool // Set when installed
}

// Status describes what the resolver determined about a command.
type Status int

const (
	// StatusBuiltin means the command is a built-in CySec command.
	StatusBuiltin Status = iota

	// StatusInstalled means the tool is installed and ready to execute.
	StatusInstalled

	// StatusAvailable means the tool is in the registry but not installed.
	StatusAvailable

	// StatusUnsupported means the tool exists but isn't supported on this platform.
	StatusUnsupported

	// StatusDeprecated means the tool is deprecated with a replacement.
	StatusDeprecated

	// StatusUnknown means the command wasn't found anywhere.
	StatusUnknown
)

// String returns a human-readable status description.
func (s Status) String() string {
	switch s {
	case StatusBuiltin:
		return "built-in command"
	case StatusInstalled:
		return "installed"
	case StatusAvailable:
		return "available (not installed)"
	case StatusUnsupported:
		return "unsupported on this platform"
	case StatusDeprecated:
		return "deprecated"
	case StatusUnknown:
		return "not found"
	default:
		return "unknown"
	}
}

// BuiltinCommands lists all CySec.env built-in command names.
var BuiltinCommands = map[string]bool{
	"help":      true,
	"banner":    true,
	"tools":     true,
	"search":    true,
	"info":      true,
	"install":   true,
	"uninstall": true,
	"update":    true,
	"registry":  true,
	"doctor":    true,
	"storage":   true,
	"cache":     true,
	"workspace": true,
	"vpn":       true,
	"version":   true,
}

// Resolver resolves commands through the pipeline.
type Resolver struct {
	db       *database.StateDB
	reg      *registry.Registry
	osName   string
	archName string
}

// New creates a new Resolver.
func New(db *database.StateDB, reg *registry.Registry, osName, archName string) *Resolver {
	return &Resolver{
		db:       db,
		reg:      reg,
		osName:   osName,
		archName: archName,
	}
}

// Resolve determines what a command refers to.
func (r *Resolver) Resolve(command string) *Resolution {
	// 1. Check built-in commands
	if BuiltinCommands[command] {
		return &Resolution{Status: StatusBuiltin}
	}

	// 2. Check installed tools
	installed, err := r.db.Get(command)
	if err == nil && installed != nil {
		// Also look up registry entry for full metadata
		regEntry := r.reg.Lookup(command)
		return &Resolution{
			Status:        StatusInstalled,
			Tool:          regEntry,
			InstalledTool: installed,
		}
	}

	// 3. Check registry
	regEntry := r.reg.Lookup(command)
	if regEntry != nil {
		// Check if deprecated
		if regEntry.Deprecated {
			return &Resolution{
				Status: StatusDeprecated,
				Tool:   regEntry,
			}
		}

		// Check platform support
		if !regEntry.IsSupported(r.osName, r.archName) {
			return &Resolution{
				Status: StatusUnsupported,
				Tool:   regEntry,
			}
		}

		return &Resolution{
			Status: StatusAvailable,
			Tool:   regEntry,
		}
	}

	// 4. Unknown
	return &Resolution{Status: StatusUnknown}
}

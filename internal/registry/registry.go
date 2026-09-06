// Package registry defines the tool registry schema and provides loading,
// querying, and filtering of registry tool entries.
package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// ToolEntry represents a single tool in the CySec.env registry.
type ToolEntry struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	DisplayName string   `yaml:"display_name"`
	Aliases     []string `yaml:"aliases,omitempty"`

	Categories  []string `yaml:"categories"`
	Description string   `yaml:"description"`

	Profiles            []string           `yaml:"profiles,omitempty"`
	InstallClass        string             `yaml:"install_class,omitempty"`        // PREINSTALLED, REGISTRY_ONLY, PLATFORM_SPECIFIC, SYSTEM_INTEGRATED
	RuntimeRequirements []string           `yaml:"runtime_requirements,omitempty"` // e.g. ["python3", "java17", "go"]
	InterfaceType       string             `yaml:"interface_type,omitempty"`
	HostPrerequisites   []HostPrerequisite `yaml:"host_prerequisites,omitempty"`

	OfficialWebsite       string `yaml:"official_website,omitempty"`
	OfficialRepository    string `yaml:"official_repository,omitempty"`
	OfficialReleaseSource string `yaml:"official_release_source,omitempty"`

	License string `yaml:"license,omitempty"`

	Platforms     []PlatformSpec `yaml:"platforms"`
	Architectures []string       `yaml:"architectures,omitempty"`

	InstallationMethod string `yaml:"installation_method"`
	InstallerAdapter   string `yaml:"installer_adapter"`

	EntryCommand string   `yaml:"entry_command"`
	Dependencies []string `yaml:"dependencies,omitempty"`

	Version       string `yaml:"version"`
	VersionPolicy string `yaml:"version_policy,omitempty"`

	Checksum          string `yaml:"checksum,omitempty"`
	ChecksumURL       string `yaml:"checksum_url,omitempty"`
	ChecksumAlgorithm string `yaml:"checksum_algorithm,omitempty"`
	Signature         string `yaml:"signature,omitempty"`

	VerificationStatus string `yaml:"verification_status"`
	ToolStatus         string `yaml:"tool_status"`

	EstimatedDownloadSize  string `yaml:"estimated_download_size,omitempty"`
	EstimatedInstalledSize string `yaml:"estimated_installed_size,omitempty"`

	LastVerified string `yaml:"last_verified,omitempty"`

	Deprecated  bool   `yaml:"deprecated,omitempty"`
	Replacement string `yaml:"replacement,omitempty"`

	// Adapter-specific configuration
	AdapterConfig map[string]string `yaml:"adapter_config,omitempty"`
}

// HostPrerequisite defines OS-level dependencies like Java or Npcap.
type HostPrerequisite struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Command     string `yaml:"command"`
	Mandatory   bool   `yaml:"mandatory"`
}

// PlatformSpec describes support for a specific platform.
type PlatformSpec struct {
	OS      string `yaml:"os"`
	Arch    string `yaml:"arch,omitempty"`
	Support string `yaml:"support"` // SUPPORTED, LIMITED, EXPERIMENTAL, UNSUPPORTED
	Notes   string `yaml:"notes,omitempty"`

	// Platform-specific download URL for binary adapter
	DownloadURL       string `yaml:"download_url,omitempty"`
	Checksum          string `yaml:"checksum,omitempty"`
	ChecksumURL       string `yaml:"checksum_url,omitempty"`
	ChecksumAlgorithm string `yaml:"checksum_algorithm,omitempty"`
}

// Registry holds all loaded tool entries and provides query methods.
type Registry struct {
	tools           map[string]*ToolEntry
	aliasIndex      map[string]string // alias -> tool ID
	schemaVer       string
	registryDir     string
	loadedPaths     map[string]bool // abs filepath -> loaded
	duplicateIDs    int
	aliasCollisions int
}

// New creates a new empty Registry.
func New(registryDir string) *Registry {
	return &Registry{
		tools:       make(map[string]*ToolEntry),
		aliasIndex:  make(map[string]string),
		registryDir: registryDir,
		loadedPaths: make(map[string]bool),
	}
}

// LoadEmbedded loads registry entries from the embedded registry directory in the binary.
func LoadEmbedded(embeddedDir string) (*Registry, error) {
	r := New(embeddedDir)
	return r, r.loadFromDir(embeddedDir)
}

// LoadFromDir loads all YAML tool files from a directory.
func (r *Registry) LoadFromDir(dir string) error {
	return r.loadFromDir(dir)
}

func (r *Registry) loadFromDir(dir string) error {
	toolsDir := filepath.Join(dir, "tools")
	if _, err := os.Stat(toolsDir); os.IsNotExist(err) {
		toolsDir = dir // Try the directory itself
	}

	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No registry files yet
		}
		return fmt.Errorf("failed to read registry directory %s: %w", toolsDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}

		filePath := filepath.Join(toolsDir, entry.Name())
		absPath, err := filepath.Abs(filePath)
		if err == nil {
			if runtime.GOOS == "windows" {
				absPath = strings.ToLower(absPath)
			}
			if r.loadedPaths[absPath] {
				continue // Already loaded this exact file
			}
			r.loadedPaths[absPath] = true
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		var tool ToolEntry
		if err := yaml.Unmarshal(data, &tool); err != nil {
			fmt.Fprintf(os.Stderr, "Registry warning:\nFile: %s\nStatus: SKIPPED\nReason: failed to parse YAML: %v\n\n", filePath, err)
			continue
		}

		if tool.ID == "" {
			tool.ID = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		}

		if tool.InstallationMethod == "" && tool.InstallerAdapter == "" {
			fmt.Fprintf(os.Stderr, "Registry warning:\nFile: %s\nStatus: SKIPPED\nReason: missing required fields \"installation_method\" and \"installer_adapter\"\n\n", filePath)
			continue
		}

		if _, exists := r.tools[tool.ID]; exists {
			r.duplicateIDs++
			fmt.Fprintf(os.Stderr, "Registry warning:\nFile: %s\nStatus: SKIPPED\nReason: duplicate tool ID %q\n\n", filePath, tool.ID)
			continue
		}

		r.tools[tool.ID] = &tool

		// Index aliases
		for _, alias := range tool.Aliases {
			lowerAlias := strings.ToLower(alias)
			if existingID, exists := r.aliasIndex[lowerAlias]; exists && existingID != tool.ID {
				r.aliasCollisions++
				fmt.Fprintf(os.Stderr, "Registry warning:\nFile: %s\nStatus: SKIPPED ALIAS\nReason: duplicate alias %q already points to %q\n\n", filePath, alias, existingID)
				continue
			}
			r.aliasIndex[lowerAlias] = tool.ID
		}
		// Also index by entry command
		if tool.EntryCommand != "" {
			lowerCmd := strings.ToLower(tool.EntryCommand)
			if existingID, exists := r.aliasIndex[lowerCmd]; exists && existingID != tool.ID {
				r.aliasCollisions++
				fmt.Fprintf(os.Stderr, "Registry warning:\nFile: %s\nStatus: SKIPPED ENTRY COMMAND ALIAS\nReason: duplicate entry command alias %q already points to %q\n\n", filePath, tool.EntryCommand, existingID)
			} else {
				r.aliasIndex[lowerCmd] = tool.ID
			}
		}
	}

	return nil
}

// Lookup finds a tool by ID, name, or alias.
func (r *Registry) Lookup(query string) *ToolEntry {
	q := strings.ToLower(query)

	// Direct ID match
	if tool, ok := r.tools[q]; ok {
		return tool
	}

	// Alias match
	if id, ok := r.aliasIndex[q]; ok {
		if tool, ok := r.tools[id]; ok {
			return tool
		}
	}

	// Name match (case-insensitive)
	for _, tool := range r.tools {
		if strings.EqualFold(tool.Name, query) || strings.EqualFold(tool.DisplayName, query) {
			return tool
		}
	}

	return nil
}

// Search finds tools matching a query string in name, description, or categories.
func (r *Registry) Search(query string) []*ToolEntry {
	q := strings.ToLower(query)
	var results []*ToolEntry

	for _, tool := range r.tools {
		if matches(tool, q) {
			results = append(results, tool)
		}
	}

	return results
}

// matches checks if a tool matches a search query.
func matches(tool *ToolEntry, query string) bool {
	if strings.Contains(strings.ToLower(tool.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(tool.DisplayName), query) {
		return true
	}
	if strings.Contains(strings.ToLower(tool.Description), query) {
		return true
	}
	for _, alias := range tool.Aliases {
		if strings.Contains(strings.ToLower(alias), query) {
			return true
		}
	}
	for _, cat := range tool.Categories {
		if strings.Contains(strings.ToLower(cat), query) {
			return true
		}
	}
	return false
}

// ByCategory returns all tools in a given category.
func (r *Registry) ByCategory(category string) []*ToolEntry {
	cat := strings.ToLower(category)
	var results []*ToolEntry

	for _, tool := range r.tools {
		for _, c := range tool.Categories {
			if strings.EqualFold(c, cat) || strings.Contains(strings.ToLower(c), cat) {
				results = append(results, tool)
				break
			}
		}
	}

	return results
}

// ByProfile returns all tools belonging to a given profile.
func (r *Registry) ByProfile(profile string) []*ToolEntry {
	prof := strings.ToLower(profile)
	var results []*ToolEntry

	for _, tool := range r.tools {
		for _, p := range tool.Profiles {
			if strings.EqualFold(p, prof) {
				results = append(results, tool)
				break
			}
		}
	}

	return results
}

// All returns all tools in the registry.
func (r *Registry) All() []*ToolEntry {
	result := make([]*ToolEntry, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// Categories returns a list of all unique categories.
func (r *Registry) Categories() []string {
	seen := make(map[string]bool)
	var cats []string
	for _, tool := range r.tools {
		for _, c := range tool.Categories {
			lower := strings.ToLower(c)
			if !seen[lower] {
				seen[lower] = true
				cats = append(cats, c)
			}
		}
	}
	return cats
}

// Count returns the number of tools in the registry.
func (r *Registry) Count() int {
	return len(r.tools)
}

// GetPlatformSpec returns the platform spec for the given OS and arch, if any.
func (t *ToolEntry) GetPlatformSpec(osName, arch string) *PlatformSpec {
	for i, p := range t.Platforms {
		if strings.EqualFold(p.OS, osName) {
			if p.Arch == "" || strings.EqualFold(p.Arch, arch) {
				return &t.Platforms[i]
			}
		}
	}
	return nil
}

// IsSupported returns true if the tool is supported on the given OS.
func (t *ToolEntry) IsSupported(osName, arch string) bool {
	spec := t.GetPlatformSpec(osName, arch)
	if spec == nil {
		return false
	}
	return spec.Support == "SUPPORTED" || spec.Support == "LIMITED" || spec.Support == "EXPERIMENTAL"
}

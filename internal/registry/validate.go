package registry

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidationResult represents the structured outcome of a registry validation run.
type ValidationResult struct {
	TotalEntries   int
	ValidEntries   int
	InvalidEntries int
	DuplicateIDs   int
	AliasCollisions int
	Errors         []string
	Warnings       []string
}

// HasErrors returns true if any critical validation errors were found.
func (v *ValidationResult) HasErrors() bool {
	return len(v.Errors) > 0
}

// ValidateAll performs a comprehensive static analysis of all loaded tools in the registry.
func (r *Registry) ValidateAll() *ValidationResult {
	res := &ValidationResult{
		TotalEntries:    len(r.tools) + r.duplicateIDs,
		DuplicateIDs:    r.duplicateIDs,
		AliasCollisions: r.aliasCollisions,
	}

	// 1. Cross-tool validations (Duplicates are partially handled during load, 
	// but we track them here or just re-validate).
	// During loadFromDir, we skipped duplicates, so they aren't in r.tools. 
	// To properly count DuplicateIDs, we would need to track them during load, 
	// but for now, we just validate what is loaded. 

	for _, tool := range r.tools {
		errors := ValidateTool(tool)
		if len(errors) > 0 {
			res.InvalidEntries++
			for _, err := range errors {
				res.Errors = append(res.Errors, fmt.Sprintf("[%s] %s", tool.ID, err))
			}
		} else {
			res.ValidEntries++
		}
	}

	return res
}

// ValidateTool checks a single tool entry for strict schema and lifecycle compliance.
func ValidateTool(tool *ToolEntry) []string {
	var errs []string

	if tool.ID == "" {
		errs = append(errs, "missing tool ID")
	}
	if tool.Name == "" {
		errs = append(errs, "missing name")
	}
	if tool.DisplayName == "" {
		errs = append(errs, "missing display_name")
	}
	if len(tool.Categories) == 0 {
		errs = append(errs, "missing categories")
	}

	// Adapter validation
	if tool.InstallerAdapter == "" {
		errs = append(errs, "missing installer_adapter")
	} else if tool.InstallerAdapter != "binary" && tool.InstallerAdapter != "go_module" && tool.InstallerAdapter != "python" && tool.InstallerAdapter != "system_wrapper" {
		errs = append(errs, fmt.Sprintf("invalid installer_adapter: %q", tool.InstallerAdapter))
	}

	// URL validation
	if tool.OfficialWebsite != "" && !isValidURL(tool.OfficialWebsite) {
		errs = append(errs, "invalid official_website URL format")
	}
	if tool.OfficialRepository != "" && !isValidURL(tool.OfficialRepository) {
		errs = append(errs, "invalid official_repository URL format")
	}

	// Interface Type validation
	if tool.InterfaceType != "" && tool.InterfaceType != "cli" && tool.InterfaceType != "gui" {
		errs = append(errs, fmt.Sprintf("invalid interface_type: %q", tool.InterfaceType))
	}

	// Install Class validation
	if tool.InstallClass != "" {
		validClasses := map[string]bool{
			"PREINSTALLED":      true,
			"REGISTRY_ONLY":     true,
			"PLATFORM_SPECIFIC": true,
			"SYSTEM_INTEGRATED": true,
		}
		if !validClasses[strings.ToUpper(tool.InstallClass)] {
			errs = append(errs, fmt.Sprintf("invalid install_class: %q", tool.InstallClass))
		}
	}

	// Status validation
	validStatuses := map[string]bool{
		"EXPERIMENTAL": true,
		"DRAFT":        true,
		"VALIDATED":    true,
		"SUPPORTED":    true,
		"DEPRECATED":   true,
		"UNSUPPORTED":  true,
	}
	if tool.ToolStatus == "" {
		errs = append(errs, "missing tool_status")
	} else if !validStatuses[strings.ToUpper(tool.ToolStatus)] {
		errs = append(errs, fmt.Sprintf("invalid tool_status: %q", tool.ToolStatus))
	}

	// Checksums metadata
	if tool.ChecksumURL != "" && !isValidURL(tool.ChecksumURL) {
		errs = append(errs, "invalid checksum_url format")
	}

	// Platform validation
	if len(tool.Platforms) == 0 {
		errs = append(errs, "no platforms defined")
	}
	for _, p := range tool.Platforms {
		if p.OS == "" {
			errs = append(errs, "platform missing os")
		}
		if p.Support != "SUPPORTED" && p.Support != "LIMITED" && p.Support != "EXPERIMENTAL" && p.Support != "UNSUPPORTED" {
			errs = append(errs, fmt.Sprintf("invalid platform support value: %q", p.Support))
		}
		if p.DownloadURL != "" && !isValidURL(p.DownloadURL) {
			errs = append(errs, fmt.Sprintf("invalid platform download_url for %s", p.OS))
		}
	}

	return errs
}

func isValidURL(u string) bool {
	parsed, err := url.ParseRequestURI(u)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return true
}

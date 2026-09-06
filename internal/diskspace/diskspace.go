// Package diskspace provides disk space estimation and availability checking
// for CySec.env installation planning.
package diskspace

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cysec-env/cysec/internal/registry"
)

// Estimate holds the calculated disk space requirements for an installation.
type Estimate struct {
	DownloadSize    int64 // Total download size in bytes
	InstalledSize   int64 // Total installed size in bytes
	TempSize        int64 // Temporary extraction/build space
	RuntimeSize     int64 // Shared runtime/dependency size
	SafetyMargin    int64 // 10% safety margin
	TotalRequired   int64 // Sum of all above
	AvailableSpace  int64 // Available disk space at target
	Sufficient      bool  // Whether available >= required

	// Breakdown by install class
	PreinstalledCount    int
	RegistryOnlyCount    int
	PlatformSpecificCount int
	SystemIntegratedCount int
	UnsupportedCount     int
	TotalCount           int
}

// EstimateProfile calculates disk space requirements for a set of tools.
func EstimateProfile(tools []*registry.ToolEntry, osName, arch string) *Estimate {
	est := &Estimate{}
	est.TotalCount = len(tools)

	for _, tool := range tools {
		// Classify
		switch strings.ToUpper(tool.InstallClass) {
		case "PREINSTALLED":
			est.PreinstalledCount++
		case "REGISTRY_ONLY":
			est.RegistryOnlyCount++
			continue // Don't count disk for registry-only tools
		case "PLATFORM_SPECIFIC":
			est.PlatformSpecificCount++
			if !tool.IsSupported(osName, arch) {
				est.UnsupportedCount++
				continue
			}
		case "SYSTEM_INTEGRATED":
			est.SystemIntegratedCount++
			continue // System tools don't consume CySec disk space
		default:
			est.PreinstalledCount++ // Default to preinstalled
		}

		dlSize := parseSizeString(tool.EstimatedDownloadSize)
		instSize := parseSizeString(tool.EstimatedInstalledSize)

		est.DownloadSize += dlSize
		est.InstalledSize += instSize
	}

	// Temporary space is estimated as max of download size (archives in transit)
	est.TempSize = est.DownloadSize

	// Runtime space estimate (Python, Go, Java runtimes)
	est.RuntimeSize = 500 * 1024 * 1024 // ~500 MB baseline for shared runtimes

	// Safety margin: 10% of total
	subtotal := est.InstalledSize + est.TempSize + est.RuntimeSize
	est.SafetyMargin = subtotal / 10
	est.TotalRequired = subtotal + est.SafetyMargin

	return est
}

// FormatSize formats bytes into human-readable format.
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// parseSizeString parses strings like "25 MB", "1.5 GB", "500 KB" into bytes.
func parseSizeString(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "TBD" || s == "0" {
		return 0
	}

	// Match number (possibly decimal) followed by optional unit
	re := regexp.MustCompile(`^([\d.]+)\s*(GB|MB|KB|B)?$`)
	matches := re.FindStringSubmatch(strings.ToUpper(s))
	if len(matches) < 2 {
		return 0
	}

	val, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	unit := "MB" // default
	if len(matches) >= 3 && matches[2] != "" {
		unit = matches[2]
	}

	switch unit {
	case "GB":
		return int64(val * 1024 * 1024 * 1024)
	case "MB":
		return int64(val * 1024 * 1024)
	case "KB":
		return int64(val * 1024)
	default:
		return int64(val)
	}
}

// CheckAvailable wraps the platform-specific getAvailableSpace function.
func CheckAvailable(path string) (int64, error) {
	return getAvailableSpace(path)
}

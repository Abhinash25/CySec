// Package verify provides checksum and integrity verification for downloads.
package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cysec-env/cysec/internal/registry"
)

type TrustLevel string

const (
	TrustOfficialVerified  TrustLevel = "OFFICIAL_VERIFIED"
	TrustIntegrityVerified TrustLevel = "INTEGRITY_VERIFIED"
	TrustOfficialSource    TrustLevel = "OFFICIAL_SOURCE"
	TrustCommunityKnown    TrustLevel = "COMMUNITY_KNOWN"
	TrustUnverified        TrustLevel = "UNVERIFIED"
	TrustInvalid           TrustLevel = "INVALID"
)

// EvaluateTrust determines the trust level of a tool entry.
func EvaluateTrust(entry *registry.ToolEntry) TrustLevel {
	if entry == nil {
		return TrustInvalid
	}

	if entry.VerificationStatus == "OFFICIAL_VERIFIED" {
		return TrustOfficialVerified
	}

	hasChecksum := false
	if entry.Checksum != "" {
		hasChecksum = true
	} else {
		for _, p := range entry.Platforms {
			if p.Checksum != "" {
				hasChecksum = true
				break
			}
		}
	}

	if hasChecksum {
		return TrustIntegrityVerified
	}

	if entry.OfficialReleaseSource != "" || entry.OfficialRepository != "" {
		return TrustOfficialSource
	}

	if len(entry.Categories) > 0 {
		return TrustCommunityKnown
	}

	return TrustUnverified
}

// Result represents a verification outcome.
type Result struct {
	Passed     bool
	Expected   string
	Actual     string
	Method     string
	Skipped    bool
	SkipReason string
}

// SHA256File computes the SHA256 hash of a file.
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file for hashing: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyChecksum verifies a file against an expected SHA256 checksum.
func VerifyChecksum(filePath, expectedChecksum string) (*Result, error) {
	if expectedChecksum == "" {
		return &Result{
			Passed:     true,
			Skipped:    true,
			SkipReason: "No checksum provided in registry",
			Method:     "sha256",
		}, nil
	}

	// Strip algorithm prefix if present (e.g., "sha256:abc123")
	expected := expectedChecksum
	if strings.Contains(expected, ":") {
		parts := strings.SplitN(expected, ":", 2)
		expected = parts[1]
	}
	expected = strings.ToLower(strings.TrimSpace(expected))

	actual, err := SHA256File(filePath)
	if err != nil {
		return nil, err
	}

	return &Result{
		Passed:   actual == expected,
		Expected: expected,
		Actual:   actual,
		Method:   "sha256",
	}, nil
}

// String returns a human-readable verification result.
func (r *Result) String() string {
	if r.Skipped {
		return fmt.Sprintf("Verification skipped: %s", r.SkipReason)
	}
	if r.Passed {
		return fmt.Sprintf("Verification passed (%s)", r.Method)
	}
	return fmt.Sprintf("Verification FAILED (%s): expected %s, got %s", r.Method, r.Expected, r.Actual)
}

// FetchAndExtractChecksum downloads a checksum file and strictly resolves the hash for the given filename.
func FetchAndExtractChecksum(url string, targetFilename string, algo string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch checksum metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d fetching checksum metadata", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read checksum metadata: %w", err)
	}

	return ParseChecksumsStrict(string(content), targetFilename, algo)
}

// ParseChecksumsStrict parses a checksums file and strictly extracts the hash for the target filename.
func ParseChecksumsStrict(content string, targetFilename string, algo string) (string, error) {
	if algo == "" {
		algo = "sha256"
	}
	algo = strings.ToLower(algo)

	lines := strings.Split(content, "\n")
	var matchedHash string
	matchCount := 0

	// Regex for SHA256(filename)= hash OR hash  filename OR hash *filename
	// Common formats:
	// 5c2...  nuclei_3.0_linux_amd64.zip
	// 5c2... *nuclei_3.0_linux_amd64.zip
	// SHA256 (nuclei_3.0_linux_amd64.zip) = 5c2...
	// SHA256(nuclei_3.0_linux_amd64.zip)= 5c2...

	reCoreutils := regexp.MustCompile(`^([a-fA-F0-9]+)\s+[\* ]?(.+)$`)
	reOpenSSL := regexp.MustCompile(`(?i)^[A-Z0-9-]+\s*\((.+)\)\s*=\s*([a-fA-F0-9]+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var parsedHash, parsedFile string

		if match := reCoreutils.FindStringSubmatch(line); match != nil {
			parsedHash = match[1]
			parsedFile = strings.TrimSpace(match[2])
		} else if match := reOpenSSL.FindStringSubmatch(line); match != nil {
			parsedFile = strings.TrimSpace(match[1])
			parsedHash = match[2]
		} else {
			continue
		}

		// Strictly match the target filename exactly (ignoring path if present in the checksum file)
		if filepath.Base(parsedFile) == targetFilename || parsedFile == targetFilename {
			matchedHash = parsedHash
			matchCount++
		}
	}

	if matchCount == 0 {
		return "", fmt.Errorf("no exact match found for artifact %s in checksum metadata", targetFilename)
	}
	if matchCount > 1 {
		return "", fmt.Errorf("ambiguous checksum resolution: %d matches found for artifact %s", matchCount, targetFilename)
	}

	// Validate the extracted value is a valid hash based on algorithm
	matchedHash = strings.ToLower(matchedHash)
	if algo == "sha256" {
		if len(matchedHash) != 64 {
			return "", fmt.Errorf("resolved checksum is not a valid SHA-256 hash (length %d)", len(matchedHash))
		}
	}

	return matchedHash, nil
}

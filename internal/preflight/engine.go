package preflight

import (
	"os/exec"
	"runtime"
	"strings"
	"fmt"

	"github.com/cysec-env/cysec/internal/registry"
)

// CheckResult represents the outcome of a single prerequisite check.
type CheckResult struct {
	Prerequisite registry.HostPrerequisite
	Present      bool
	Output       string
	Error        error
}

// Check evaluates a single host prerequisite.
func Check(req registry.HostPrerequisite) CheckResult {
	res := CheckResult{
		Prerequisite: req,
	}

	if req.Command == "" {
		res.Present = true // Implicitly true if no command is specified (e.g. documentation only)
		return res
	}

	// We support two modes: 
	// 1. Simple executable lookup (if it's a single word with no spaces like "java")
	// 2. Shell command evaluation (if it contains spaces)
	
	if !strings.Contains(req.Command, " ") {
		path, err := exec.LookPath(req.Command)
		if err == nil {
			res.Present = true
			res.Output = fmt.Sprintf("Found at: %s", path)
		} else {
			res.Present = false
			res.Error = err
		}
		return res
	}

	// Shell evaluation
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", req.Command)
	} else {
		cmd = exec.Command("sh", "-c", req.Command)
	}

	out, err := cmd.CombinedOutput()
	res.Output = strings.TrimSpace(string(out))

	if err != nil {
		res.Present = false
		res.Error = err
	} else {
		res.Present = true
	}

	return res
}

// CheckAll evaluates all prerequisites for a given tool.
func CheckAll(tool *registry.ToolEntry) []CheckResult {
	var results []CheckResult
	for _, req := range tool.HostPrerequisites {
		results = append(results, Check(req))
	}
	return results
}

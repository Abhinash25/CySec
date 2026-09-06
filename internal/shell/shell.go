// Package shell implements the CySec.env interactive command shell.
// It provides a custom REPL that intercepts CySec-specific commands
// (tools, search, info, install, etc.) and passes all other commands
// through to the underlying OS shell (PowerShell/bash).
package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/cysec-env/cysec/internal/database"
	"github.com/cysec-env/cysec/internal/installer"
	"github.com/cysec-env/cysec/internal/platform"
	"github.com/cysec-env/cysec/internal/registry"
	"github.com/cysec-env/cysec/internal/shim"
	"github.com/cysec-env/cysec/internal/ui"
	"github.com/cysec-env/cysec/internal/verify"
)

// Shell is the CySec interactive command shell.
type Shell struct {
	platform *platform.Info
	registry *registry.Registry
	db       *database.StateDB
	engine   *installer.Engine
	shims    *shim.Manager
	envPath  string
}

// New creates a new interactive shell.
func New(pinfo *platform.Info, reg *registry.Registry, db *database.StateDB, engine *installer.Engine, shims *shim.Manager) *Shell {
	shimDir := pinfo.ShimDir
	currentPath := os.Getenv("PATH")
	newPath := shimDir + string(os.PathListSeparator) + currentPath
	return &Shell{
		platform: pinfo,
		registry: reg,
		db:       db,
		engine:   engine,
		shims:    shims,
		envPath:  newPath,
	}
}

type commandHandler func(s *Shell, args []string)

var builtinCommands map[string]commandHandler

func init() {
	builtinCommands = map[string]commandHandler{
		"help":      (*Shell).cmdHelp,
		"tools":     (*Shell).cmdTools,
		"list":      (*Shell).cmdTools,
		"search":    (*Shell).cmdSearch,
		"info":      (*Shell).cmdInfo,
		"install":   (*Shell).cmdInstall,
		"verify":    (*Shell).cmdVerify,
		"remove":    (*Shell).cmdRemove,
		"uninstall": (*Shell).cmdRemove,
		"status":    (*Shell).cmdStatus,
		"doctor":    (*Shell).cmdDoctor,
	}
}

// IsBuiltin returns true if the command is a CySec built-in.
func IsBuiltin(cmd string) bool {
	_, ok := builtinCommands[cmd]
	return ok || cmd == "exit" || cmd == "quit"
}

// Run starts the interactive REPL. It blocks until the user types "exit".
func (s *Shell) Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		cwd, _ := os.Getwd()
		leaf := filepath.Base(cwd)
		fmt.Printf("cysec@env:%s$ ", leaf)

		if !scanner.Scan() {
			fmt.Println()
			return nil
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := tokenize(line)
		if len(parts) == 0 {
			continue
		}

		cmd := strings.ToLower(parts[0])
		args := parts[1:]

		if cmd == "exit" || cmd == "quit" {
			return nil
		}

		if handler, ok := builtinCommands[cmd]; ok {
			handler(s, args)
			continue
		}

		// Support "cysec <subcommand>" typed inside the shell
		if cmd == "cysec" && len(args) > 0 {
			subcmd := strings.ToLower(args[0])
			if subcmd == "exit" || subcmd == "quit" {
				return nil
			}
			if handler, ok := builtinCommands[subcmd]; ok {
				handler(s, args[1:])
				continue
			}
		}

		s.execPassthrough(line)
	}
}

func tokenize(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(c)
			}
		} else {
			if c == '"' || c == '\'' {
				inQuote = true
				quoteChar = c
			} else if c == ' ' || c == '\t' {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			} else {
				current.WriteByte(c)
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func (s *Shell) execPassthrough(line string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		sh := s.platform.Shell()
		if strings.Contains(sh, "pwsh") || strings.Contains(sh, "powershell") {
			cmd = exec.Command(sh, "-NoProfile", "-Command", line)
		} else {
			cmd = exec.Command("cmd.exe", "/C", line)
		}
	} else {
		shellPath := os.Getenv("SHELL")
		if shellPath == "" {
			shellPath = "/bin/sh"
		}
		cmd = exec.Command(shellPath, "-c", line)
	}

	env := os.Environ()
	env = setEnv(env, "PATH", s.envPath)
	env = setEnv(env, "CYSEC_ENV", "1")
	env = setEnv(env, "CYSEC_HOME", s.platform.DataDir)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		}
	}
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), strings.ToUpper(prefix)) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// ---- Command Handlers ----

func (s *Shell) cmdHelp(args []string) {
	fmt.Println()
	fmt.Println(ui.StyleBold.Render("CySec.env Commands"))
	fmt.Println(ui.StyleDim.Render(strings.Repeat("\u2500", 40)))
	fmt.Println()
	cmds := []struct {
		name string
		desc string
	}{
		{"help", "Show this help message"},
		{"tools", "List installed tools"},
		{"tools --all", "List all registry tools with status"},
		{"list", "Alias for tools"},
		{"list --all", "Alias for tools --all"},
		{"search <query>", "Search registry by name/category/description"},
		{"info <tool>", "Show detailed tool information"},
		{"install <tool>", "Install a tool from the registry"},
		{"verify <tool>", "Verify a tool's installation health"},
		{"remove <tool>", "Remove an installed tool"},
		{"status", "Show environment status"},
		{"doctor", "Run health checks on all components"},
		{"exit", "Exit the CySec.env shell"},
	}
	for _, c := range cmds {
		padded := c.name + strings.Repeat(" ", maxInt(1, 22-len(c.name)))
		fmt.Printf("  %s%s\n", ui.StyleInfo.Render(padded), c.desc)
	}
	fmt.Println()
	fmt.Println(ui.StyleDim.Render("  Any other command is passed through to the OS shell."))
	fmt.Println(ui.StyleDim.Render("  Example: nmap, jq --version, curl, docker ps"))
	fmt.Println()
}

func (s *Shell) cmdTools(args []string) {
	showAll := false
	for _, a := range args {
		if a == "--all" || a == "-a" {
			showAll = true
		}
	}
	if showAll {
		s.toolsAll()
	} else {
		s.toolsInstalled()
	}
}

func (s *Shell) toolsInstalled() {
	installed, err := s.db.ListInstalled()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to list installed tools: %v", err))
		return
	}
	if len(installed) == 0 {
		ui.Info("No tools installed yet. Use 'install <tool>' to install a tool.")
		return
	}
	ui.Section("Installed Tools")
	headers := []string{"Name", "Version", "Adapter", "Health"}
	var rows [][]string
	for _, t := range installed {
		health := t.HealthStatus
		switch health {
		case "healthy":
			health = ui.StyleSuccess.Render("healthy")
		case "broken":
			health = ui.StyleError.Render("broken")
		default:
			health = ui.StyleWarning.Render("unknown")
		}
		rows = append(rows, []string{t.Name, t.Version, t.Adapter, health})
	}
	ui.Table(headers, rows)
	fmt.Println()
}

func (s *Shell) toolsAll() {
	tools := s.registry.All()
	if len(tools) == 0 {
		ui.Warn("No tools found in registry.")
		return
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].DisplayName < tools[j].DisplayName
	})
	ui.Section("CySec.env Tool Registry")
	fmt.Printf("  %d tools in registry\n\n", len(tools))
	headers := []string{"Name", "Category", "Status", "Adapter"}
	var rows [][]string
	for _, t := range tools {
		status := s.resolveToolStatus(t)
		cat := ""
		if len(t.Categories) > 0 {
			cat = t.Categories[0]
		}
		rows = append(rows, []string{t.DisplayName, cat, status, t.InstallerAdapter})
	}
	ui.Table(headers, rows)
	fmt.Println()
}

func (s *Shell) resolveToolStatus(t *registry.ToolEntry) string {
	if s.db.IsInstalled(t.ID) {
		installed, _ := s.db.Get(t.ID)
		if installed != nil && installed.HealthStatus == "broken" {
			return ui.StyleError.Render("BROKEN")
		}
		if installed != nil && installed.Ownership == "SYSTEM_MANAGED" {
			return ui.StyleSuccess.Render("SYSTEM_INTEGRATED")
		}
		return ui.StyleSuccess.Render("INSTALLED")
	}
	if !t.IsSupported(runtime.GOOS, runtime.GOARCH) {
		return ui.StyleWarning.Render("PLATFORM_REQUIRED")
	}
	return "AVAILABLE"
}

func (s *Shell) cmdSearch(args []string) {
	if len(args) == 0 {
		ui.Error("Usage: search <query>")
		return
	}
	query := strings.Join(args, " ")
	results := s.registry.Search(query)
	if len(results) == 0 {
		ui.Warn(fmt.Sprintf("No tools found matching '%s'", query))
		ui.Info("Try 'tools --all' to list all tools.")
		return
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].DisplayName < results[j].DisplayName
	})
	ui.Section(fmt.Sprintf("Search Results for '%s'", query))
	fmt.Printf("  %d matches found\n\n", len(results))
	headers := []string{"Name", "Category", "Status", "Adapter"}
	var rows [][]string
	for _, t := range results {
		status := s.resolveToolStatus(t)
		cat := ""
		if len(t.Categories) > 0 {
			cat = t.Categories[0]
		}
		rows = append(rows, []string{t.DisplayName, cat, status, t.InstallerAdapter})
	}
	ui.Table(headers, rows)
	fmt.Println()
}

func (s *Shell) cmdInfo(args []string) {
	if len(args) == 0 {
		ui.Error("Usage: info <tool>")
		return
	}
	toolName := args[0]
	tool := s.registry.Lookup(toolName)
	if tool == nil {
		ui.Error(fmt.Sprintf("Tool '%s' not found in registry.", toolName))
		ui.Info("Use 'search <query>' to find tools.")
		return
	}
	ui.Section(tool.DisplayName)
	width := 24
	ui.KeyValue("ID:", tool.ID, width)
	ui.KeyValue("Description:", tool.Description, width)
	ui.KeyValue("Categories:", strings.Join(tool.Categories, ", "), width)
	if len(tool.Aliases) > 0 {
		ui.KeyValue("Aliases:", strings.Join(tool.Aliases, ", "), width)
	}
	ui.KeyValue("License:", tool.License, width)
	ui.KeyValue("Version:", tool.Version, width)
	ui.KeyValue("Entry Command:", tool.EntryCommand, width)
	ui.KeyValue("Install Method:", tool.InstallationMethod, width)
	ui.KeyValue("Adapter:", tool.InstallerAdapter, width)
	if tool.OfficialWebsite != "" {
		ui.KeyValue("Website:", tool.OfficialWebsite, width)
	}
	if tool.OfficialRepository != "" {
		ui.KeyValue("Repository:", tool.OfficialRepository, width)
	}
	fmt.Println()
	fmt.Println(ui.StyleBold.Render("  Registry Metadata:"))
	ui.KeyValue("    Trust Level:", string(verify.EvaluateTrust(tool)), width)
	ui.KeyValue("    Lifecycle Status:", tool.ToolStatus, width)
	if tool.EstimatedDownloadSize != "" {
		ui.KeyValue("    Download Size:", tool.EstimatedDownloadSize, width)
	}
	fmt.Println()
	fmt.Println(ui.StyleBold.Render("  Platform Support:"))
	for _, p := range tool.Platforms {
		pstatus := p.Support
		switch pstatus {
		case "SUPPORTED":
			pstatus = ui.StyleSuccess.Render(pstatus)
		case "LIMITED", "EXPERIMENTAL":
			pstatus = ui.StyleWarning.Render(pstatus)
		case "UNSUPPORTED":
			pstatus = ui.StyleError.Render(pstatus)
		}
		arch := p.Arch
		if arch == "" {
			arch = "all"
		}
		current := ""
		if p.OS == runtime.GOOS {
			current = " <- current"
		}
		fmt.Printf("    %s %s/%s  %s%s\n", ui.SymbolBullet, p.OS, arch, pstatus, ui.StyleDim.Render(current))
		if p.Notes != "" {
			fmt.Printf("      %s\n", ui.StyleDim.Render(p.Notes))
		}
	}
	fmt.Println()
	fmt.Println(ui.StyleBold.Render("  Installation Status:"))
	if s.db.IsInstalled(tool.ID) {
		installed, _ := s.db.Get(tool.ID)
		if installed != nil {
			fmt.Println(ui.StyleSuccess.Render("    \u2713 Installed"))
			ui.KeyValue("    Version:", installed.Version, width)
			ui.KeyValue("    Path:", installed.InstallPath, width)
			ui.KeyValue("    Health:", installed.HealthStatus, width)
		}
	} else {
		fmt.Printf("    %s Not installed. Use 'install %s' to install.\n", ui.StyleDim.Render("\u25cb"), tool.ID)
	}
	fmt.Println()
}

func (s *Shell) cmdInstall(args []string) {
	if len(args) == 0 {
		ui.Error("Usage: install <tool>")
		return
	}
	toolName := args[0]
	tool := s.registry.Lookup(toolName)
	if tool == nil {
		ui.Error(fmt.Sprintf("Tool '%s' not found in registry.", toolName))
		ui.Info("Use 'search <query>' to find tools.")
		return
	}
	if s.db.IsInstalled(tool.ID) {
		installed, _ := s.db.Get(tool.ID)
		if installed != nil && installed.HealthStatus == "healthy" {
			ui.Success(fmt.Sprintf("%s is already installed and healthy.", tool.DisplayName))
			return
		}
	}
	if !tool.IsSupported(runtime.GOOS, runtime.GOARCH) {
		ui.Error(fmt.Sprintf("Tool '%s' is not supported on %s/%s.", tool.DisplayName, runtime.GOOS, runtime.GOARCH))
		return
	}
	ui.Section(fmt.Sprintf("Install %s", tool.DisplayName))
	width := 22
	ui.KeyValue("Description:", tool.Description, width)
	if len(tool.Categories) > 0 {
		ui.KeyValue("Category:", tool.Categories[0], width)
	}
	ui.KeyValue("Version:", tool.Version, width)
	ui.KeyValue("Adapter:", tool.InstallerAdapter, width)
	ui.KeyValue("Trust Level:", string(verify.EvaluateTrust(tool)), width)
	fmt.Println()
	validationErrors := registry.ValidateTool(tool)
	if len(validationErrors) > 0 {
		ui.Error(fmt.Sprintf("Cannot install %s: Registry entry is invalid.", tool.DisplayName))
		for _, errStr := range validationErrors {
			fmt.Printf("  %s %s\n", ui.StyleError.Render("\u2717"), errStr)
		}
		return
	}
	plan, err := s.engine.Plan(tool)
	if err != nil {
		ui.Error(fmt.Sprintf("Cannot plan installation: %v", err))
		return
	}
	fmt.Println(ui.StyleBold.Render("  Installation Steps:"))
	for _, step := range plan.Steps {
		fmt.Printf("    %s %s\n", ui.SymbolArrow, step)
	}
	if !ui.Confirm("Install now?", true) {
		ui.Info("Installation cancelled.")
		return
	}
	fmt.Println()
	result, err := s.engine.Install(tool, s.db, s.shims)
	if err != nil {
		ui.Error(fmt.Sprintf("Installation failed: %v", err))
		return
	}
	if result.EntryPoint != "" {
		shimName := tool.EntryCommand
		if shimName == "" {
			shimName = tool.ID
		}
		_, shimErr := s.shims.Create(shimName, result.EntryPoint, tool.InterfaceType)
		if shimErr != nil {
			ui.Warn(fmt.Sprintf("Failed to create command shim: %v", shimErr))
		}
	}
	fmt.Println()
	ui.Success(fmt.Sprintf("%s installed successfully.", tool.DisplayName))
	if tool.EntryCommand != "" {
		ui.Info(fmt.Sprintf("Run '%s' to use it.", tool.EntryCommand))
	}
	fmt.Println()
}

func (s *Shell) cmdVerify(args []string) {
	if len(args) == 0 {
		ui.Error("Usage: verify <tool>")
		return
	}
	toolName := args[0]
	tool := s.registry.Lookup(toolName)
	if tool == nil {
		ui.Error(fmt.Sprintf("Tool '%s' not found in registry.", toolName))
		return
	}
	ui.Section(fmt.Sprintf("Verify %s", tool.DisplayName))
	if !s.db.IsInstalled(tool.ID) {
		fmt.Printf("  Status: %s\n", ui.StyleWarning.Render("NOT_INSTALLED"))
		ui.Info(fmt.Sprintf("Use 'install %s' to install it first.", tool.ID))
		fmt.Println()
		return
	}
	installed, _ := s.db.Get(tool.ID)
	fmt.Printf("  Registry: %s\n", ui.StyleSuccess.Render("REGISTRY_VALIDATED"))
	if installed != nil && installed.InstallPath != "" {
		if _, err := os.Stat(installed.InstallPath); err == nil {
			fmt.Printf("  Files:    %s\n", ui.StyleSuccess.Render("INSTALLED"))
		} else {
			fmt.Printf("  Files:    %s\n", ui.StyleError.Render("BROKEN"))
			s.db.UpdateHealth(tool.ID, "broken")
			fmt.Println()
			return
		}
	}
	err := s.engine.HealthCheck(tool)
	if err != nil {
		fmt.Printf("  Execute:  %s\n", ui.StyleError.Render("BROKEN"))
		fmt.Printf("  Detail:   %s\n", ui.StyleDim.Render(err.Error()))
		s.db.UpdateHealth(tool.ID, "broken")
	} else {
		fmt.Printf("  Execute:  %s\n", ui.StyleSuccess.Render("EXECUTION_VERIFIED"))
		s.db.UpdateHealth(tool.ID, "healthy")
	}
	fmt.Println()
}

func (s *Shell) cmdRemove(args []string) {
	if len(args) == 0 {
		ui.Error("Usage: remove <tool>")
		return
	}
	toolName := args[0]
	installed, err := s.db.Get(toolName)
	if err != nil || installed == nil {
		ui.Error(fmt.Sprintf("Tool '%s' is not installed.", toolName))
		return
	}
	if installed.Ownership == "SYSTEM_MANAGED" {
		ui.Error(fmt.Sprintf("Cannot remove '%s': it is SYSTEM_MANAGED.", installed.Name))
		ui.Info("System-integrated tools must be managed through your operating system's package manager.")
		return
	}
	ui.Section(fmt.Sprintf("Remove %s", installed.Name))
	ui.KeyValue("Version:", installed.Version, 22)
	ui.KeyValue("Path:", installed.InstallPath, 22)
	if !ui.Confirm("Remove this tool?", false) {
		ui.Info("Removal cancelled.")
		return
	}
	fmt.Println()
	shimName := installed.EntryCommand
	if shimName == "" {
		shimName = installed.ID
	}
	s.shims.Remove(shimName)
	ui.Step("Removed command shim")
	tool := s.registry.Lookup(toolName)
	if tool != nil {
		if err := s.engine.Uninstall(tool, s.db); err != nil {
			ui.Error(fmt.Sprintf("Uninstall failed: %v", err))
			return
		}
	} else {
		s.db.Remove(toolName)
	}
	ui.Step("Removed tool files")
	fmt.Println()
	ui.Success(fmt.Sprintf("%s has been removed.", installed.Name))
	fmt.Println()
}

func (s *Shell) cmdStatus(args []string) {
	profile, _ := s.db.GetMeta("setup_profile")
	if profile == "" {
		profile = "CUSTOM"
	} else {
		profile = strings.ToUpper(profile)
	}
	installed, _ := s.db.ListInstalled()
	installedCount := len(installed)
	regCount := s.registry.Count()
	brokenCount := 0
	for _, t := range installed {
		if t.HealthStatus == "broken" {
			brokenCount++
		}
	}
	ui.Section("CySec.env Status")
	fmt.Println()
	width := 26
	ui.KeyValue("Profile:", profile, width)
	ui.KeyValue("Platform:", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), width)
	ui.KeyValue("Registry Tools:", fmt.Sprintf("%d", regCount), width)
	ui.KeyValue("Installed Tools:", fmt.Sprintf("%d", installedCount), width)
	if brokenCount > 0 {
		ui.KeyValue("Broken Tools:", ui.StyleError.Render(fmt.Sprintf("%d", brokenCount)), width)
	} else {
		ui.KeyValue("Broken Tools:", "0", width)
	}
	ui.KeyValue("CySec Home:", s.platform.DataDir, width)
	ui.KeyValue("Workspace:", s.platform.WorkspaceDir(), width)
	ui.KeyValue("Tools Directory:", s.platform.ToolsDir(), width)
	ui.KeyValue("Registry Directory:", s.platform.RegistryDir(), width)
	fmt.Println()
}

func (s *Shell) cmdDoctor(args []string) {
	ui.Section("CySec.env Health Report")
	fmt.Println()
	warnings := 0
	errors := 0
	checkResult("Core Application", true, "")
	checkResult("Platform", true, fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
	regCount := s.registry.Count()
	if regCount > 0 {
		checkResult("Registry", true, fmt.Sprintf("%d tools", regCount))
	} else {
		checkResult("Registry", false, "empty")
		warnings++
	}
	dbCount := s.db.Count()
	checkResult("Database", true, fmt.Sprintf("%d installed", dbCount))
	fmt.Println()
	fmt.Println(ui.StyleBold.Render("  Runtime Availability:"))
	rts := []struct {
		name string
		cmds []string
	}{
		{"Git", []string{"git"}},
		{"Python", []string{"python3", "python"}},
		{"Go", []string{"go"}},
		{"Node.js", []string{"node"}},
		{"Docker", []string{"docker"}},
	}
	for _, rt := range rts {
		found := false
		for _, cmd := range rt.cmds {
			if platform.HasCommand(cmd) {
				found = true
				break
			}
		}
		if found {
			checkResult(rt.name, true, "")
		} else {
			checkResult(rt.name, false, "not found")
		}
	}
	installed, _ := s.db.ListInstalled()
	if len(installed) > 0 {
		fmt.Println()
		fmt.Println(ui.StyleBold.Render("  Installed Tools:"))
		broken := 0
		for _, t := range installed {
			tool := s.registry.Lookup(t.ID)
			if tool != nil {
				err := s.engine.HealthCheck(tool)
				if err != nil {
					checkResult(t.Name, false, "health check failed")
					s.db.UpdateHealth(t.ID, "broken")
					broken++
				} else {
					checkResult(t.Name, true, t.Version)
					s.db.UpdateHealth(t.ID, "healthy")
				}
			} else {
				checkResult(t.Name, true, t.Version)
			}
		}
		if broken > 0 {
			errors += broken
		}
	}
	fmt.Println()
	if errors == 0 && warnings == 0 {
		ui.Success("All checks passed.")
	} else {
		if errors > 0 {
			ui.Error(fmt.Sprintf("%d errors found.", errors))
		}
		if warnings > 0 {
			ui.Warn(fmt.Sprintf("%d warnings.", warnings))
		}
	}
	fmt.Println()
}

func checkResult(name string, ok bool, detail string) {
	if ok {
		if detail != "" {
			fmt.Printf("    %s %-24s %s\n", ui.StyleSuccess.Render(ui.SymbolCheck), name, ui.StyleDim.Render(detail))
		} else {
			fmt.Printf("    %s %s\n", ui.StyleSuccess.Render(ui.SymbolCheck), name)
		}
	} else {
		if detail != "" {
			fmt.Printf("    %s %-24s %s\n", ui.StyleError.Render(ui.SymbolCross), name, ui.StyleWarning.Render(detail))
		} else {
			fmt.Printf("    %s %s\n", ui.StyleError.Render(ui.SymbolCross), name)
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

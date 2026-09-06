// Package ui provides terminal UI helpers for CySec.env including styled
// output, tables, progress indicators, and user confirmations.
package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette — curated cybersecurity theme.
var (
	ColorCyan    = lipgloss.Color("#00D4FF")
	ColorGreen   = lipgloss.Color("#00FF88")
	ColorYellow  = lipgloss.Color("#FFD700")
	ColorRed     = lipgloss.Color("#FF4444")
	ColorOrange  = lipgloss.Color("#FF8C00")
	ColorMagenta = lipgloss.Color("#FF00FF")
	ColorDim     = lipgloss.Color("#666666")
	ColorWhite   = lipgloss.Color("#FFFFFF")
	ColorBlue    = lipgloss.Color("#4488FF")
)

// Styles
var (
	// Title style for section headers
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorCyan)

	// Success indicator
	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorGreen)

	// Warning indicator
	StyleWarning = lipgloss.NewStyle().
			Foreground(ColorYellow)

	// Error indicator
	StyleError = lipgloss.NewStyle().
			Foreground(ColorRed)

	// Dim text for secondary information
	StyleDim = lipgloss.NewStyle().
			Foreground(ColorDim)

	// Bold white for emphasis
	StyleBold = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite)

	// Category / tag style
	StyleTag = lipgloss.NewStyle().
			Foreground(ColorMagenta)

	// Info / link style
	StyleInfo = lipgloss.NewStyle().
			Foreground(ColorBlue)

	// Banner style
	StyleBanner = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	// Box style for bordered sections
	StyleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorCyan).
			Padding(1, 2)
)

// Banner returns the CySec.env ASCII art banner.
func Banner() string {
	art := `
        ██████╗██╗   ██╗███████╗███████╗ ██████╗
       ██╔════╝╚██╗ ██╔╝██╔════╝██╔════╝██╔════╝
       ██║      ╚████╔╝ ███████╗█████╗  ██║     
       ██║       ╚██╔╝  ╚════██║██╔══╝  ██║     
       ╚██████╗   ██║   ███████║███████╗╚██████╗
        ╚═════╝   ╚═╝   ╚══════╝╚══════╝ ╚═════╝`

	tagline := "\n                    CySec.env\n\n             One Environment. Every Tool.\n"

	return StyleBanner.Render(art) + "\n" + StyleDim.Render(tagline)
}

// BannerWithStatus returns the CySec.env banner with dynamic environment status.
func BannerWithStatus(profile string, installedCount int) string {
	if profile == "" {
		profile = "CUSTOM"
	} else {
		profile = strings.ToUpper(profile)
	}

	art := `
        ██████╗██╗   ██╗███████╗███████╗ ██████╗
       ██╔════╝╚██╗ ██╔╝██╔════╝██╔════╝██╔════╝
       ██║      ╚████╔╝ ███████╗█████╗  ██║     
       ██║       ╚██╔╝  ╚════██║██╔══╝  ██║     
       ╚██████╗   ██║   ███████║███████╗╚██████╗
        ╚═════╝   ╚═╝   ╚══════╝╚══════╝ ╚═════╝`

	title := "\n                    CySec.env\n"
	
	status := fmt.Sprintf("\nEnvironment: %s\nTools: %d installed\nStatus: %s\n", 
		StyleCyan(profile), 
		installedCount, 
		StyleGreen("READY"))
	
	return StyleBanner.Render(art) + StyleTitle.Render(title) + status
}

// Style helpers
func StyleCyan(s string) string { return lipgloss.NewStyle().Foreground(ColorCyan).Render(s) }
func StyleGreen(s string) string { return lipgloss.NewStyle().Foreground(ColorGreen).Render(s) }

// Symbols for status output.
const (
	SymbolCheck   = "✓"
	SymbolCross   = "✗"
	SymbolWarning = "⚠"
	SymbolArrow   = "→"
	SymbolBullet  = "•"
	SymbolInfo    = "ℹ"
)

// Success prints a success message.
func Success(msg string) {
	fmt.Println(StyleSuccess.Render(SymbolCheck) + " " + msg)
}

// Warn prints a warning message.
func Warn(msg string) {
	fmt.Println(StyleWarning.Render(SymbolWarning) + " " + msg)
}

// Error prints an error message.
func Error(msg string) {
	fmt.Println(StyleError.Render(SymbolCross) + " " + msg)
}

// Info prints an info message.
func Info(msg string) {
	fmt.Println(StyleInfo.Render(SymbolInfo) + " " + msg)
}

// Step prints a step indicator (used during install/update flows).
func Step(msg string) {
	fmt.Println(StyleDim.Render(SymbolArrow) + " " + msg)
}

// Section prints a titled section.
func Section(title string) {
	fmt.Println()
	fmt.Println(StyleTitle.Render(title))
	fmt.Println(StyleDim.Render(strings.Repeat("─", len(title)+2)))
}

// KeyValue prints a key-value pair with aligned formatting.
func KeyValue(key, value string, width int) {
	padded := key + strings.Repeat(" ", max(1, width-len(key)))
	fmt.Println("  " + StyleDim.Render(padded) + value)
}

// Table prints a simple table with headers and rows.
func Table(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	headerLine := ""
	separator := ""
	for i, h := range headers {
		padded := h + strings.Repeat(" ", widths[i]-len(h))
		headerLine += StyleBold.Render(padded)
		separator += strings.Repeat("─", widths[i])
		if i < len(headers)-1 {
			headerLine += "  "
			separator += "──"
		}
	}
	fmt.Println("  " + headerLine)
	fmt.Println("  " + StyleDim.Render(separator))

	// Print rows
	for _, row := range rows {
		line := ""
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			padded := cell + strings.Repeat(" ", widths[i]-len(cell))
			line += padded
			if i < len(row)-1 {
				line += "  "
			}
		}
		fmt.Println("  " + line)
	}
}

// Confirm asks the user a yes/no question. Returns true for yes.
func Confirm(prompt string, defaultYes bool) bool {
	suffix := "[Y/n]"
	if !defaultYes {
		suffix = "[y/N]"
	}

	fmt.Printf("\n%s %s ", prompt, StyleDim.Render(suffix))

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes"
}

// ConfirmExplicit requires the user to type a specific phrase to confirm.
func ConfirmExplicit(prompt, requiredPhrase string) bool {
	fmt.Printf("\n%s\n", prompt)
	fmt.Printf("Type %s to proceed: ", StyleWarning.Render(requiredPhrase))

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	return input == requiredPhrase
}

// ProgressStage represents a named installation stage.
type ProgressStage struct {
	Name   string
	Status string // "pending", "active", "done", "failed"
}

// RenderProgress displays a list of stages with status indicators.
func RenderProgress(stages []ProgressStage) {
	for _, s := range stages {
		var icon string
		switch s.Status {
		case "done":
			icon = StyleSuccess.Render(SymbolCheck)
		case "active":
			icon = StyleInfo.Render(SymbolArrow)
		case "failed":
			icon = StyleError.Render(SymbolCross)
		default:
			icon = StyleDim.Render(SymbolBullet)
		}
		fmt.Printf("  %s %s\n", icon, s.Name)
	}
}

// FormatSize formats bytes into a human-readable string.
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

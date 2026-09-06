package shell

import (
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"help", []string{"help"}},
		{"tools --all", []string{"tools", "--all"}},
		{"search wireshark", []string{"search", "wireshark"}},
		{"info wireshark", []string{"info", "wireshark"}},
		{"install wireshark", []string{"install", "wireshark"}},
		{`search "network analysis"`, []string{"search", "network analysis"}},
		{"  tools   --all  ", []string{"tools", "--all"}},
		{"", nil},
		{"exit", []string{"exit"}},
		{"jq --version", []string{"jq", "--version"}},
		{"nmap -sV 10.0.0.1", []string{"nmap", "-sV", "10.0.0.1"}},
		{"cysec doctor", []string{"cysec", "doctor"}},
	}

	for _, tt := range tests {
		result := tokenize(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("tokenize(%q) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i, v := range result {
			if v != tt.expected[i] {
				t.Errorf("tokenize(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
			}
		}
	}
}

func TestIsBuiltin(t *testing.T) {
	builtins := []string{"help", "tools", "list", "search", "info", "install", "verify", "remove", "uninstall", "status", "doctor", "exit", "quit"}
	for _, cmd := range builtins {
		if !IsBuiltin(cmd) {
			t.Errorf("IsBuiltin(%q) = false, want true", cmd)
		}
	}

	notBuiltins := []string{"nmap", "jq", "docker", "curl", "wireshark", "python", "unknowncmd123"}
	for _, cmd := range notBuiltins {
		if IsBuiltin(cmd) {
			t.Errorf("IsBuiltin(%q) = true, want false", cmd)
		}
	}
}

func TestTokenizeQuotedStrings(t *testing.T) {
	result := tokenize(`search 'network scanner'`)
	if len(result) != 2 || result[0] != "search" || result[1] != "network scanner" {
		t.Errorf("single-quoted tokenize failed: %v", result)
	}

	result = tokenize(`search "web vulnerability"`)
	if len(result) != 2 || result[0] != "search" || result[1] != "web vulnerability" {
		t.Errorf("double-quoted tokenize failed: %v", result)
	}
}

func TestTokenizeEmptyAndWhitespace(t *testing.T) {
	result := tokenize("")
	if len(result) != 0 {
		t.Errorf("empty string should return nil, got %v", result)
	}

	result = tokenize("   ")
	if len(result) != 0 {
		t.Errorf("whitespace-only should return nil, got %v", result)
	}
}

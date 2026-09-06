package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type ToolDef struct {
	ID                 string
	Name               string
	DisplayName        string
	Category           string
	Adapter            string
	InstallClass       string
	RuntimeRequirements []string
	Profiles           []string
}

func main() {
	tools := []ToolDef{
		// Recon / OSINT
		{"masscan", "masscan", "Masscan", "Recon / OSINT", "system_wrapper", "SYSTEM_INTEGRATED", nil, []string{"full"}},
		{"shodan", "shodan", "Shodan CLI", "Recon / OSINT", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"recon-ng", "recon-ng", "Recon-ng", "Recon / OSINT", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"maltego", "maltego", "Maltego CE", "Recon / OSINT", "system_wrapper", "SYSTEM_INTEGRATED", []string{"java17"}, []string{"full"}},
		{"photon", "photon", "Photon", "Recon / OSINT", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"censys", "censys", "Censys CLI", "Recon / OSINT", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"whatweb", "whatweb", "WhatWeb", "Recon / OSINT", "python", "PREINSTALLED", []string{"ruby"}, []string{"full"}},

		// Network Discovery & Scanning
		{"rustscan", "rustscan", "RustScan", "Network Discovery & Scanning", "binary", "PREINSTALLED", nil, []string{"full"}},
		{"zmap", "zmap", "ZMap", "Network Discovery & Scanning", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},
		{"unicornscan", "unicornscan", "Unicornscan", "Network Discovery & Scanning", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},
		{"hping3", "hping3", "Hping3", "Network Discovery & Scanning", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},
		{"arpscan", "arp-scan", "ARP-Scan", "Network Discovery & Scanning", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},
		{"dnsrecon", "dnsrecon", "DNSRecon", "Network Discovery & Scanning", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"fierce", "fierce", "Fierce", "Network Discovery & Scanning", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"dnsx", "dnsx", "Dnsx", "Network Discovery & Scanning", "go_module", "PREINSTALLED", []string{"go"}, []string{"full"}},

		// Web Application Security
		{"nikto", "nikto", "Nikto", "Web Application Security", "python", "PREINSTALLED", []string{"perl"}, []string{"full"}},
		{"wpscan", "wpscan", "WPScan", "Web Application Security", "system_wrapper", "REGISTRY_ONLY", []string{"ruby"}, []string{"full"}},
		{"dalfox", "dalfox", "Dalfox", "Web Application Security", "go_module", "PREINSTALLED", []string{"go"}, []string{"full"}},
		{"arjun", "arjun", "Arjun", "Web Application Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"paramspider", "paramspider", "ParamSpider", "Web Application Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"xsstrike", "xsstrike", "XSStrike", "Web Application Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"commix", "commix", "Commix", "Web Application Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"wfuzz", "wfuzz", "Wfuzz", "Web Application Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},

		// API Security
		{"postman", "postman", "Postman CLI", "API Security", "binary", "REGISTRY_ONLY", nil, []string{"full"}},
		{"mitmproxy", "mitmproxy", "Mitmproxy", "API Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"httpie", "httpie", "HTTPie", "API Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},

		// Network Security
		{"ngrep", "ngrep", "Ngrep", "Network Security", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},
		{"ettercap", "ettercap", "Ettercap", "Network Security", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},
		{"responder", "responder", "Responder", "Network Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"socat", "socat", "Socat", "Network Security", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},

		// Password / Credential Auditing
		{"hydra", "hydra", "Hydra", "Password / Credential Auditing", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},
		{"medusa", "medusa", "Medusa", "Password / Credential Auditing", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},
		{"cewl", "cewl", "CeWL", "Password / Credential Auditing", "python", "PREINSTALLED", []string{"ruby"}, []string{"full"}},
		{"hash-identifier", "hash-identifier", "Hash-identifier", "Password / Credential Auditing", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},

		// Wireless Security
		{"aircrack-ng", "aircrack-ng", "Aircrack-ng", "Wireless Security", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},
		{"kismet", "kismet", "Kismet", "Wireless Security", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},
		{"wifite", "wifite", "Wifite", "Wireless Security", "python", "PLATFORM_SPECIFIC", []string{"python3"}, []string{"full"}},
		{"fern-wifi-cracker", "fern-wifi-cracker", "Fern Wifi Cracker", "Wireless Security", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},

		// Exploitation / Security Testing
		{"metasploit", "msfconsole", "Metasploit Framework", "Exploitation / Security Testing", "system_wrapper", "SYSTEM_INTEGRATED", []string{"ruby"}, []string{"full"}},
		{"searchsploit", "searchsploit", "SearchSploit", "Exploitation / Security Testing", "system_wrapper", "REGISTRY_ONLY", nil, []string{"full"}},
		{"routersploit", "routersploit", "RouterSploit", "Exploitation / Security Testing", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"pwntools", "pwn", "Pwntools", "Exploitation / Security Testing", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"ropgadget", "ROPgadget", "ROPgadget", "Exploitation / Security Testing", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"cme", "cme", "CrackMapExec", "Exploitation / Security Testing", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"evil-winrm", "evil-winrm", "Evil-WinRM", "Exploitation / Security Testing", "system_wrapper", "REGISTRY_ONLY", []string{"ruby"}, []string{"full"}},
		{"covenant", "covenant", "Covenant", "Exploitation / Security Testing", "system_wrapper", "REGISTRY_ONLY", []string{"dotnet"}, []string{"full"}},

		// Active Directory / Windows Security
		{"bloodhound", "bloodhound", "BloodHound", "Active Directory / Windows Security", "binary", "PREINSTALLED", nil, []string{"full"}},
		{"impacket", "impacket", "Impacket", "Active Directory / Windows Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"kerbrute", "kerbrute", "Kerbrute", "Active Directory / Windows Security", "go_module", "PREINSTALLED", []string{"go"}, []string{"full"}},
		{"rubeus", "Rubeus", "Rubeus", "Active Directory / Windows Security", "binary", "REGISTRY_ONLY", nil, []string{"full"}},
		{"sharphound", "SharpHound", "SharpHound", "Active Directory / Windows Security", "binary", "REGISTRY_ONLY", nil, []string{"full"}},
		{"enum4linux-ng", "enum4linux-ng", "Enum4linux-ng", "Active Directory / Windows Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},

		// Cloud Security
		{"scoutsuite", "scout", "ScoutSuite", "Cloud Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"prowler", "prowler", "Prowler", "Cloud Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"cloudmapper", "cloudmapper", "CloudMapper", "Cloud Security", "python", "REGISTRY_ONLY", []string{"python3"}, []string{"full"}},
		{"pacu", "pacu", "Pacu", "Cloud Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"cloudfox", "cloudfox", "Cloudfox", "Cloud Security", "go_module", "PREINSTALLED", []string{"go"}, []string{"full"}},
		{"trufflehog", "trufflehog", "Trufflehog", "Cloud Security", "go_module", "PREINSTALLED", []string{"go"}, []string{"full"}},

		// Container / Kubernetes Security
		{"trivy", "trivy", "Trivy", "Container / Kubernetes Security", "binary", "PREINSTALLED", nil, []string{"full"}},
		{"grype", "grype", "Grype", "Container / Kubernetes Security", "binary", "PREINSTALLED", nil, []string{"full"}},
		{"kubeaudit", "kubeaudit", "Kubeaudit", "Container / Kubernetes Security", "go_module", "PREINSTALLED", []string{"go"}, []string{"full"}},
		{"kube-hunter", "kube-hunter", "Kube-hunter", "Container / Kubernetes Security", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"docker-bench", "docker-bench-security", "Docker Bench", "Container / Kubernetes Security", "system_wrapper", "REGISTRY_ONLY", nil, []string{"full"}},

		// Reverse Engineering
		{"ghidra", "ghidraRun", "Ghidra", "Reverse Engineering", "binary", "PREINSTALLED", []string{"java17"}, []string{"full"}},
		{"radare2", "r2", "Radare2", "Reverse Engineering", "binary", "PREINSTALLED", nil, []string{"full"}},
		{"binaryninja", "binaryninja", "Binary Ninja", "Reverse Engineering", "system_wrapper", "REGISTRY_ONLY", nil, []string{"full"}},
		{"cutter", "cutter", "Cutter", "Reverse Engineering", "binary", "REGISTRY_ONLY", nil, []string{"full"}},
		{"angr", "angr", "Angr", "Reverse Engineering", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},

		// Forensics / DFIR
		{"volatility3", "vol", "Volatility3", "Forensics / DFIR", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"autopsy", "autopsy", "Autopsy", "Forensics / DFIR", "system_wrapper", "SYSTEM_INTEGRATED", []string{"java17"}, []string{"full"}},
		{"binwalk", "binwalk", "Binwalk", "Forensics / DFIR", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		{"foremost", "foremost", "Foremost", "Forensics / DFIR", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},
		{"yara", "yara", "YARA", "Forensics / DFIR", "binary", "PREINSTALLED", nil, []string{"full"}},
		{"exiftool", "exiftool", "ExifTool", "Forensics / DFIR", "binary", "PREINSTALLED", []string{"perl"}, []string{"full"}},

		// Security Analysis / Utilities
		{"cyberchef", "cyberchef", "CyberChef", "Security Analysis / Utilities", "binary", "PREINSTALLED", nil, []string{"full"}},
		{"jq", "jq", "jq", "Security Analysis / Utilities", "binary", "PREINSTALLED", nil, []string{"full", "core"}},
		{"curl", "curl", "curl", "Security Analysis / Utilities", "system_wrapper", "SYSTEM_INTEGRATED", nil, []string{"full", "core"}},
		{"wget", "wget", "wget", "Security Analysis / Utilities", "binary", "PREINSTALLED", nil, []string{"full"}},
		{"stegsolve", "stegsolve", "Stegsolve", "Security Analysis / Utilities", "binary", "REGISTRY_ONLY", []string{"java17"}, []string{"full"}},
		{"binutils", "binutils", "Binutils", "Security Analysis / Utilities", "system_wrapper", "PLATFORM_SPECIFIC", nil, []string{"full"}},
		{"openssl", "openssl", "OpenSSL", "Security Analysis / Utilities", "system_wrapper", "SYSTEM_INTEGRATED", nil, []string{"full"}},
		{"seclists", "seclists", "SecLists", "Security Analysis / Utilities", "binary", "PREINSTALLED", nil, []string{"full"}},
		{"payloadsallthethings", "payloadsallthethings", "PayloadsAllTheThings", "Security Analysis / Utilities", "binary", "REGISTRY_ONLY", nil, []string{"full"}},
		{"wordlistctl", "wordlistctl", "Wordlistctl", "Security Analysis / Utilities", "python", "PREINSTALLED", []string{"python3"}, []string{"full"}},
		
		// Supporting Runtimes (Add these too)
		{"java17", "java", "Java JRE 17+", "Runtimes", "system_wrapper", "SYSTEM_INTEGRATED", nil, []string{"full", "core"}},
		{"go", "go", "Go Toolchain", "Runtimes", "binary", "SYSTEM_INTEGRATED", nil, []string{"full", "core"}},
	}

	template := `id: %s
name: %s
display_name: %s

aliases:
  - %s

categories:
  - %s

description: "TBD"

install_class: %s
%s
profiles:
%s
installation_method: %s
installer_adapter: %s

entry_command: %s

version: "latest"
version_policy: "stable"

verification_status: UNVERIFIED
tool_status: DRAFT

estimated_download_size: "10 MB"
estimated_installed_size: "25 MB"

platforms:
  - os: windows
    support: EXPERIMENTAL
  - os: linux
    support: EXPERIMENTAL
  - os: darwin
    support: EXPERIMENTAL
`

	outDir := filepath.Join("d:\\New\\CySec.Env\\CySec", "registry", "tools")
	
	for _, t := range tools {
		outFile := filepath.Join(outDir, t.ID+".yaml")
		
		// Check if exists
		if _, err := os.Stat(outFile); err == nil {
			fmt.Printf("Skipping %s, already exists.\n", t.ID)
			continue
		}
		
		rtReqs := ""
		if len(t.RuntimeRequirements) > 0 {
			rtReqs = "runtime_requirements:\n"
			for _, r := range t.RuntimeRequirements {
				rtReqs += fmt.Sprintf("  - %s\n", r)
			}
		}

		profs := ""
		for _, p := range t.Profiles {
			profs += fmt.Sprintf("  - %s\n", p)
		}

		installMethod := "OFFICIAL_BINARY"
		if t.Adapter == "go_module" {
			installMethod = "GO_MODULE"
		} else if t.Adapter == "python" {
			installMethod = "PYTHON_PACKAGE"
		} else if t.Adapter == "system_wrapper" {
			installMethod = "SYSTEM_PACKAGE"
		}

		content := fmt.Sprintf(template,
			t.ID,
			t.Name,
			t.DisplayName,
			t.Name,
			t.Category,
			t.InstallClass,
			rtReqs,
			profs,
			installMethod,
			t.Adapter,
			t.Name,
		)

		err := os.WriteFile(outFile, []byte(content), 0644)
		if err != nil {
			fmt.Printf("Error writing %s: %v\n", t.ID, err)
		} else {
			fmt.Printf("Created %s.yaml\n", t.ID)
		}
	}
}

# CySec.env

> **One Environment. Every Tool.**

**CySec.env** is a unified, managed terminal-first cybersecurity environment that provides a consistent and managed way of collection of security tools through a single command-line interface. Like discover, install, manage, and use authorized security tools across Windows, macOS, and Linux.


## Features

- 🛡️ Curated cybersecurity tool registry
- 💻 Terminal-only workflow
- 📦 Managed tool installation
- 🔍 Tool search and information
- 🩺 Environment health checks
- 🔧 Tool verification
- 🗂️ 111 tools available in the registry
- ⚡ Easy access through the `cysec` command
- 🪟 Windows desktop installer

## Installation

Install **CySec.env** using the desktop installer.

After installation, open CMD, PowerShell, or Windows Terminal and run:

```text
cysec

➡️CySec will enter the managed environment:

cysec@env:workspace$

You do not need to navigate to the CySec installation directory.

Basic Commands

Inside the CySec environment:

help

Show available CySec commands.

tools

List installed tools.

tools --all

List all tools in the registry and their current status.

list

Alias for tools.

list --all

Alias for tools --all.

search <tool>

Search for a tool.

Example:

search wireshark
info <tool>

Show detailed information about a tool.

Example:

info wireshark
install <tool>

Install a tool from the CySec registry.

Example:

install wireshark
verify <tool>

Check the health of an installed tool.

remove <tool>

Remove an installed CySec-managed tool.

status

Show CySec environment status.

doctor

Run environment health checks.

exit

Leave the CySec environment.

Registry

CySec uses a central registry as the source of truth for available tools.

The registry currently contains 111 tools.

Tools can have different states, including:

INSTALLED
AVAILABLE
PLATFORM_REQUIRED
SYSTEM_INTEGRATED
BROKEN
UNSUPPORTED

Registry availability does not automatically mean that a tool is installed.

Security

CySec.env is intended for authorized security testing, research, education, and defensive security work.

Only use security tools against systems and networks that you own or have explicit permission to test.

Project Status

CySec.env is currently under active development.

The Windows installation and interactive terminal environment are operational, with 111 tools represented in the registry and the current installation reporting 34 installed tools.

License

CySec.Env - One Environment. Every Tool by Gnana Lakshmi Abhinash.D
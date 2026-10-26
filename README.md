# Shuttle

[English](README.md) | [简体中文](README_zh.md)

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%2011%20%7C%2010-0078D6?logo=windows)](https://github.com/huaiqing-afk/Shuttle)
[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![Wails](https://img.shields.io/badge/Built%20with-Wails%20v2-DF0000)](https://wails.io/)
[![Release](https://img.shields.io/github/v/release/huaiqing-afk/Shuttle?include_prereleases&color=green)](https://github.com/huaiqing-afk/Shuttle/releases)

**Shuttle** is a modern, native Windows terminal workspace and environment controller designed for AI coding agents (such as **Claude Code** and **OpenAI Codex**). Built with Go, Wails v2, React, and TypeScript, it integrates authentic Windows ConPTY terminal multiplexing, intelligent dependency inventory, atomic configuration synchronization, and curated terminal themes.

---

## ✨ Features

- 🖥️ **True Windows ConPTY Terminal Integration**: Powered by native Windows ConPTY and xterm.js, providing robust interactive terminal sessions for PowerShell 7, Claude Code, and Codex CLI with full ANSI/Unicode and CJK support.
- 📑 **Session & Multi-Tab Orchestration**: Manage multiple agent workspaces simultaneously with right-click batch tab controls (Close Others, Close to the Right, Close All) and exit confirmation protection.
- 🎨 **Terminal Theme Palettes**: Includes 8 curated dark/light palettes (Tokyo Night / Day, Catppuccin Mocha / Latte, Dracula, Nord, Solarized Dark / Light) plus automatic system theme synchronization.
- 📦 **Smart Dependency Inventory & Audit**: Discovers global `npm` and `winget` installations, generates safe execution plans with preview cards, prevents unverified uninstallation, and maintains an idempotent ledger.
- ⚙️ **Atomic Configuration & Rollback**: Manages Windows Terminal profiles and PowerShell profile configurations with automated diff calculation, non-destructive merging, and one-click rollback.
- 🌐 **Zero-Dependency Bilingual UI (i18n)**: Instant switching between English and Simplified Chinese with 100% dictionary key parity.
- 📐 **Rigorous 12px Geometric Safety**: Adheres strictly to a ≥12px geometric safety boundary between interactive controls, validated across 13 combinations of viewports and DPI scalings with zero horizontal overflow.

---

## 🚀 Getting Started

### Download Prebuilt Binary

Download the latest standalone executable (`Shuttle.exe` or `Shuttle-windows-amd64.zip`) directly from [GitHub Releases](https://github.com/huaiqing-afk/Shuttle/releases). No installation required.

**Requirements**:
- Windows 10 / 11 (64-bit)
- [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) (pre-installed on Windows 11)

---

## 🛠️ Building from Source

### Prerequisites

- **Go**: 1.25 or later
- **Node.js**: 20.x or 22.x
- **Wails v2 CLI**: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0`
- **PowerShell 7** (recommended for terminal sessions)

### Build Steps

```powershell
# 1. Clone the repository
git clone https://github.com/huaiqing-afk/Shuttle.git
cd Shuttle

# 2. Build the frontend
cd frontend
npm ci
npm run build
cd ..

# 3. Build the application binary
wails build -platform windows/amd64 -clean

# The portable executable will be placed in build/bin/Shuttle.exe
.\build\bin\Shuttle.exe
```

### Live Development

```powershell
wails dev
```

---

## 🧪 Testing & Quality Gates

Run the single-command validation pipeline:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/check.ps1
```

This gate runs:
1. **Go Verification**: `gofmt`, `go vet`, and 124 unit test cases across all core/adapter packages (`go test ./...`).
2. **Frontend Verification**: TypeScript type checking (`npm run typecheck`) and production Vite compilation.
3. **Automated Geometry Gate**: Real headless Edge automation evaluating 13 viewport and DPI configurations (1080×720, 1440×900, 1920×1080 at 100%, 125%, 150% scaling, and narrow viewports down to 640×400), ensuring zero clipping, zero horizontal overflow, and ≥12px element clearance.

---

## 📚 Architecture & Contracts

- [Architecture Design & Boundaries (docs/architecture.md)](docs/architecture.md)
- [Configuration Merge Contract (docs/config-contract.md)](docs/config-contract.md)
- [Package Inventory & Safe Removal Contract (docs/uninstall-contract.md)](docs/uninstall-contract.md)
- [Bilingual Internationalization Contract (docs/i18n-contract.md)](docs/i18n-contract.md)
- [Engineering Postmortems & Lessons Learned (docs/LESSONS.md)](docs/LESSONS.md)
- [Full Verification Ledger (work/verification.md)](work/verification.md)

---

## 📄 License

This project is open-sourced under the [MIT License](LICENSE).

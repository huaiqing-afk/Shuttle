# Shuttle / 接驳

[English](README.md) | [简体中文](README_zh.md)

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%2011%20%7C%2010-0078D6?logo=windows)](https://github.com/huaiqing-afk/Shuttle)
[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![Wails](https://img.shields.io/badge/Built%20with-Wails%20v2-DF0000)](https://wails.io/)

Shuttle 是面向 AI 编程智能体（Claude Code、OpenAI Codex 等）的现代原生 Windows 终端工作台与环境管控底座。基于 Go + Wails v2 + React 打造，提供真正的 ConPTY 虚拟终端、环境感知、多会话编排、终端配色预设以及原子化的 Windows Terminal 与 PowerShell 配置托管。

---

## ✨ 核心特性

- 🖥️ **原生 Windows ConPTY 终端**：基于 Windows ConPTY 与 xterm.js，无缝执行 PowerShell 7、Claude Code 与 Codex CLI，支持真实交互、中文渲染与按键映射。
- 📑 **多会话与多标签编排**：支持多终端会话并存、Tab 批量右键关闭（关闭其他/关闭右侧/全部关闭），退出前安全确认。
- 🎨 **终端配色预设 (Palettes)**：内置 8 套经典深浅配色方案（Tokyo Night、Catppuccin、Dracula、Nord、Solarized Dark/Light 等），支持跟随系统或手动锁定。
- 📦 **智能环境与包依赖检测 (Inventory)**：自动感知本机 npm 与 winget 全局包；提供安装/卸载预览卡片、执行台账追踪与不可卸载判定保护。
- ⚙️ **配置同步与原子还原 (Configuration)**：无缝检测与合并 Windows Terminal 及 PowerShell Profile 配置，带完整 Diff 审计与一键还原。
- 🌐 **双语界面 (i18n)**：中英双语即时切换，无第三方臃肿依赖，100% 字典键值对称。
- 📐 **严格几何安全线**：所有控件间距严格遵循 ≥12px 几何约束，历经 13 组 DPI/视口（含极端窄窗）无横向溢出严格门禁。

---

## 🚀 快速开始

### 直接下载安装包

从 [GitHub Releases](https://github.com/huaiqing-afk/Shuttle/releases) 下载最新版本的 `Shuttle.exe` 或 `Shuttle-windows-amd64.zip`，解压即用（绿色免安装）。

运行条件：
- Windows 10 / 11 64-bit
- 已安装 [WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/)（Windows 11 已内置）

---

## 🛠️ 从源码构建

### 开发环境依赖

- **Go**: 1.25 或以上
- **Node.js**: 20.x 或 22.x
- **Wails CLI**: v2.17.0 (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0`)
- **PowerShell 7** (推荐)

### 构建步骤

```powershell
# 1. 克隆仓库
git clone https://github.com/huaiqing-afk/Shuttle.git
cd Shuttle

# 2. 安装前端依赖并构建
cd frontend
npm ci
npm run build
cd ..

# 3. 编译发布包
wails build -platform windows/amd64 -clean

# 产物输出于 build/bin/Shuttle.exe
.\build\bin\Shuttle.exe
```

### 开发模式

```powershell
wails dev
```

---

## 🧪 自动化测试与质量门禁

运行单命令全量门禁：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/check.ps1
```

门禁覆盖：
1. **Go 门禁**：`gofmt`、`go vet`、124 组核心逻辑单元测试（`go test ./...`）。
2. **前端门禁**：TypeScript 类型检查（`npm run typecheck`）与 Vite 生产构建。
3. **几何验收门禁**：Headless Edge 自动化运行 13 组视口与缩放组合（1080×720, 1440×900, 1920×1080 @ 100%/125%/150%，以及 864×576、720×480、720×448、640×400 极端边界），断言控件重叠度 0、横向溢出 0、相邻控件间距 ≥ 12px 且截图有效。

---

## 📄 架构与规范

- [架构设计契约 (docs/architecture.md)](docs/architecture.md)
- [配置合并规范 (docs/config-contract.md)](docs/config-contract.md)
- [包台账与卸载安全契约 (docs/uninstall-contract.md)](docs/uninstall-contract.md)
- [双语国际化契约 (docs/i18n-contract.md)](docs/i18n-contract.md)
- [工程错题集 (docs/LESSONS.md)](docs/LESSONS.md)
- [验收报告账本 (work/verification.md)](work/verification.md)

---

## 📜 许可证

本项目基于 [MIT 许可证](LICENSE) 开源。

# Drizzle Gateway for Windows (Native)

[![Release](https://img.shields.io/github/v/release/5h0ov/drizzle-windows-gateway?color=blue&label=Release)](https://github.com/5h0ov/drizzle-windows-gateway/releases/latest)
[![Platform](https://img.shields.io/badge/Platform-Windows%20x64-0078D6?logo=windows)](https://github.com/5h0ov/drizzle-windows-gateway/releases/latest)
[![RAM Usage](https://img.shields.io/badge/RAM-~430%20MB%20(vs%202%20GB%20WSL)-brightgreen)](#)

[Drizzle Gateway](https://gateway.drizzle.team) is officially distributed as Linux binaries and Docker images. Running it on Windows traditionally requires WSL2 or Docker Desktop, permanently consuming 1.5–2 GB of background RAM (`VmmemWSL`).

This project packages Drizzle Gateway into a standalone, 100% native Windows desktop app using Bun and Microsoft Edge WebView2 — zero WSL, zero Docker, and zero background memory on close.

---

## Download

Grab the standalone executable package from GitHub Releases:

**[Download Latest Windows Release (`drizzle-gateway-windows-x64.zip`)](https://github.com/5h0ov/drizzle-windows-gateway/releases/latest)**

---

## Highlights

* **100% Native (Zero WSL / Zero Docker):** Runs directly as native Windows processes. Drops to **0 MB RAM** immediately when closed.
* **System Tray Quick-Launcher:** Stays in your taskbar notification area. Right-click to launch any saved database connection (`PostgreSQL`, `MySQL`, `SQLite`, `LibSQL`) into a dedicated window.
* **Multi-Window Support:** Open multiple database windows concurrently. All instances share a single backend daemon to minimize memory.
* **Session & State Persistence:**
  * Credentials & connections: `%APPDATA%\DrizzleGateway\data\store.json`
  * Tabs, queries & cache: `%APPDATA%\DrizzleGateway\webview2\`
  * Window size & monitor position: `%APPDATA%\DrizzleGateway\window.json`
* **Windows 11 Mica & Dark Mode:** Native dark titlebar with Fluent Design Mica backdrop.

---

## Building & Updating

### Prerequisites
* [Go 1.22+](https://go.dev/dl/)
* [Bun 1.1+](https://bun.sh)

### Commands
```powershell
# Build the native application
bun run build

# Update & rebuild from latest upstream Drizzle Gateway release
bun run update
```

> **Automated Releases:** A weekly GitHub Actions workflow checks Drizzle's docs every Sunday at midnight UTC and automatically publishes new versions to Releases.

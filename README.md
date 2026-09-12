# Drizzle Gateway for Windows (100% Native)

[![Latest Release](https://img.shields.io/github/v/release/5h0ov/drizzle-windows-gateway?color=blue&label=Latest%20Release)](https://github.com/5h0ov/drizzle-windows-gateway/releases/latest)
[![Platform](https://img.shields.io/badge/Platform-Windows%20x64-0078D6?logo=windows)](https://github.com/5h0ov/drizzle-windows-gateway/releases/latest)
[![RAM Usage](https://img.shields.io/badge/RAM%20Footprint-~45%20MB-brightgreen)](#)
[![Zero WSL](https://img.shields.io/badge/WSL%20%2F%20Docker-None%20(100%25%20Native)-success)](#)

A portable, ultra-lightweight, 100% native Windows desktop application for [Drizzle Gateway](https://gateway.drizzle.team).

---

## ⚡ Quick Download

Download the ready-to-run desktop package:

👉 **[Download Latest Windows Release (`drizzle-gateway-windows-x64.zip`)](https://github.com/5h0ov/drizzle-windows-gateway/releases/latest)**

1. Extract the zip anywhere on your PC.
2. Double-click **`DrizzleGateway.exe`**.
3. That's it! No command line, no Node, no WSL, and no Docker required.

---

## ✨ Features

* **100% Windows Native (Zero WSL / Zero Docker):** Runs directly on Windows x64. No Hyper-V, no Linux VM, and **zero background WSL RAM (`VmmemWSL: 0 MB`)**.
* **Ultra-Lightweight Footprint:** Consumes only **~35–50 MB RAM** while running (compared to ~1.28 GB with WSL), and immediately drops to **0 MB** when closed.
* **Persistent Sessions & Connections (Like a Regular App):**
  * **Database Connections & Credentials:** Stored in `%APPDATA%\DrizzleGateway\data\store.json`.
  * **UI Actions, Tabs & Queries:** Table selections, open tabs, query history, and filters persist in `%APPDATA%\DrizzleGateway\webview2\`.
* **Theme Preservation:** Automatically detects Windows Dark/Light mode from the registry and applies native Immersive Dark title bars via Windows DWM (`dwmapi.dll`).
* **Auto-Centered Window:** Calculates monitor resolution and centers the window cleanly on screen.
* **Silent Process Lifecycle:** Zero flashing console or terminal popups during startup or shutdown.

---

## 📁 Project Structure

```
drizzle-windows-gateway/
├── DrizzleGateway.exe          # Native Go desktop wrapper & supervisor
├── DrizzleGatewayServer.exe    # Native background database engine
├── assets/                     # Frontend UI web bundle
│   ├── favicon-kw6xr23a.svg
│   ├── index-fca3nb0r.js
│   ├── index-mkjbygyj.css
│   └── index-p59ehkbh.html
├── scripts/
│   ├── update-gateway.bat      # 1-click local updater for new Drizzle versions
│   └── update-gateway.mjs      # Extraction & compilation script
└── .github/workflows/
    └── build-and-release.yml   # Automated CI/CD auto-update & release workflow
```

---

## 🔄 Updating to New Drizzle Gateway Versions

### Option A: Automatic Weekly CI/CD (Recommended)
This repository includes a GitHub Actions workflow that runs every Sunday at midnight UTC. It checks Drizzle's official docs for new releases (e.g. `1.7.0`), automatically builds, runs self-tests, and publishes a new release zip under [Releases](https://github.com/5h0ov/drizzle-windows-gateway/releases).

You can also trigger it manually anytime under **Actions** → **Build and Release Windows Native** → **Run workflow**.

### Option B: Local 1-Click Update
To rebuild locally on your Windows machine:
```powershell
.\scripts\update-gateway.bat 1.6.0
```
(Or pass any new version like `.\scripts\update-gateway.bat 1.7.0`). The script downloads the official binary, extracts assets, applies Windows compatibility patches, and compiles both native executables.

---

## 🛠️ Building From Source

Requirements:
* [Go 1.22+](https://go.dev/dl/)
* [Bun 1.1+](https://bun.sh)

Run:
```powershell
.\build.bat
```
Takes ~3 seconds to compile both binaries!
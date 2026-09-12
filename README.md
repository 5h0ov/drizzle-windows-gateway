# Drizzle Gateway for Windows (100% Native)

A portable, ultra-lightweight, 100% native Windows desktop application for [Drizzle Gateway](https://gateway.drizzle.team).

---

## Highlights

* **100% Windows Native (Zero WSL / Zero Docker):** Runs directly on Windows x64. No Linux VM, no Hyper-V, and **zero WSL RAM (`VmmemWSL: 0 MB`)**.
* **Zero-Config Distribution:** Simply zip the distribution files and share. Recipients just double-click `DrizzleGateway.exe`.
* **Persistent Sessions & Actions (Like a Regular App):**
  * **Database Connections & Slots:** Stored in `%APPDATA%\DrizzleGateway\data\store.json`.
  * **UI Actions, Tabs & Queries:** Table selections, open tabs, query history, and filters are stored in `%APPDATA%\DrizzleGateway\webview2\`.
* **Clean Directory Structure:** All minified web assets are neatly organized inside the `assets/` subfolder.
* **Theme Preservation:** Native Windows 10/11 Immersive Dark title bar matching system preferences.
* **Automated Updates:** Includes a 1-click update script and a GitHub Actions workflow to extract and rebuild when Drizzle releases new updates.

---

## Directory Structure

```
drizzle-windows-gateway/
├── DrizzleGateway.exe          # Native Windows desktop wrapper (Double-click to run)
├── DrizzleGatewayServer.exe    # Native background server
├── assets/                     # Frontend UI assets
│   ├── favicon-kw6xr23a.svg
│   ├── index-fca3nb0r.js
│   ├── index-mkjbygyj.css
│   └── index-p59ehkbh.html
├── scripts/
│   ├── update-gateway.bat      # 1-click updater for new Drizzle Gateway versions
│   └── update-gateway.mjs      # Bun extraction & compilation engine
└── .github/workflows/
    └── build-and-release.yml   # Automated GitHub Actions CI/CD release workflow
```

---

## How to Run

Just double-click:
👉 **`DrizzleGateway.exe`**

---

## Updating When Drizzle Releases a New Version

### Option A: Local 1-Click Update
Run:
```powershell
.\scripts\update-gateway.bat 1.6.0
```
(Or pass any new version number like `1.7.0`). The script downloads the official release, extracts the frontend bundle, applies Windows polyfills, and compiles native `.exe` files.

### Option B: Automated GitHub Actions
Push this repository to GitHub and go to **Actions** -> **Build and Release Windows Native** -> **Run workflow** (or push a git tag like `v1.6.0`). GitHub Actions will automatically compile and produce `drizzle-gateway-windows-x64.zip` in GitHub Releases.
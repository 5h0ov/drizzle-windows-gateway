# Drizzle Gateway for Windows — User Guide

A comprehensive guide to using all features of Drizzle Gateway for Windows.

---

## Table of Contents
1. [First Launch & Quick Start](#1-first-launch--quick-start)
2. [Managing Database Connections](#2-managing-database-connections)
3. [System Tray & Multi-Window Workflow](#3-system-tray--multi-window-workflow)
4. [Mobile Access & Standalone PWA](#4-mobile-access--standalone-pwa)
5. [Native Desktop Features](#5-native-desktop-features)
6. [Data Storage & Paths](#6-data-storage--paths)
7. [Troubleshooting & FAQ](#7-troubleshooting--faq)

---

## 1. First Launch & Quick Start

1. Download `drizzle-gateway-windows-x64.zip` from [Releases](https://github.com/5h0ov/drizzle-windows-gateway/releases/latest).
2. Extract the archive to any folder (e.g., `C:\Tools\DrizzleGateway` or your Desktop).
3. Double-click **`DrizzleGateway.exe`**.
4. The main window opens with a dark Fluent Design titlebar, and the Drizzle icon appears in your **Windows Taskbar Notification Area (System Tray)**.

---

## 2. Managing Database Connections

Click **"Add database connection"** to save your database credentials:

### Supported Dialects
* **PostgreSQL:** Standard Postgres, Supabase, Neon, AWS RDS, Timescale, etc.
* **MySQL:** MySQL 5.7/8.0, MariaDB, PlanetScale, AWS Aurora, etc.
* **SQLite:** Local `.sqlite` or `.db` files on your disk.
* **LibSQL:** Local or remote Turso database instances.

### Connection Modes
* **Connection String (URI):** Paste your full URI (e.g., `postgresql://user:password@host:5432/dbname?sslmode=require`).
* **Manual Fields:** Enter Host, Port, Database, User, and Password individually.
* **SSL/TLS:** Supports custom SSL root certificates, client certificates, and SSL modes (`require`, `verify-full`, etc.).

Connections and credentials are encrypted and stored in your local Windows user profile:
`%APPDATA%\DrizzleGateway\data\store.json`

---

## 3. System Tray & Multi-Window Workflow

Drizzle Gateway is designed around a single master system tray process for maximum speed and minimal memory.

### Right-Click System Tray Menu
Right-click the Drizzle Gateway icon in your taskbar notification area to access:
* **Saved Databases:** Every saved database appears as a quick-launch item. Click any database to instantly open it in a dedicated window.
* **New Empty Window:** Open a blank window to manage connections or add a new database.
* **Mobile Access:** Open the QR code pairing window for mobile access.
* **Stop Mobile Server:** Toggle the mobile proxy server on/off without closing your desktop windows.
* **Quit Drizzle Gateway:** Completely closes all windows and terminates all background processes.

### Multi-Window Productivity
* **Side-by-Side Windows:** Open multiple databases concurrently to compare schemas, inspect tables, or run queries side by side.
* **Isolated State:** Each window maintains its own navigation, search queries, and table filters without interfering with other windows.
* **Shared Backend Engine:** All windows share a single native daemon process, using only **~150–250 MB RAM** total (compared to 1.5–2 GB on Docker or WSL2).
* **Idle Memory Trimming:** When you minimize or close database windows, working-set memory automatically drops down to **~35 MB RAM**.
* **Clean Process Cleanup:** When exiting from the tray menu, an integrated Windows Job Object (`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`) ensures that all WebView2 renderers and child processes terminate immediately, dropping memory to **0 MB**.

---

## 4. Mobile Access & Standalone PWA

Access and manage your databases on your phone or tablet on the couch or on the go.

### Pairing Your Mobile Device
1. Right-click the system tray icon and click **"Mobile Access"**.
2. A pairing window will appear with an interactive QR code.

### Connection Modes

#### A. Tailscale (Anywhere in the World)
Access your databases securely from anywhere without port forwarding:
1. Ensure [Tailscale](https://tailscale.com) is installed on your PC and mobile device.
2. Drizzle Gateway **automatically detects Tailscale and activates trusted HTTPS** (`https://<machine>.<tailnet>.ts.net/`).
3. Scan the QR code with your phone camera or open the link in Chrome / Safari.
4. Tap the browser menu and select **"Install app"** (Android Chrome) or **"Add to Home Screen"** (iOS Safari).
5. **Standalone PWA Experience:** The app installs directly onto your phone as a native app with the Drizzle icon, launching in **fullscreen with zero browser address bar**.

> **Note:** The first time you use Tailscale HTTPS, ensure HTTPS Certificates are enabled in your [Tailscale DNS Settings](https://login.tailscale.com/admin/dns) (one-click toggle).

#### B. Wi-Fi (Local LAN)
When your PC and phone are connected to the same local Wi-Fi router:
1. Switch to the **Wi-Fi (Local LAN)** tab in the pairing window.
2. Scan the QR code to connect directly over your local subnet (`http://192.168.x.x:4984/`).

### Security & Token Authentication
* All mobile connections are guarded with an automated 32-character security token (`?auth=<token>`).
* Successful scans set a persistent, secure session cookie so you don't need to re-authenticate repeatedly.
* **Regenerate Token:** Click **"Regenerate"** in the pairing window to invalidate all existing mobile sessions and create a new secure QR code.
* **Toggle Server:** Turn the mobile server on or off at any time via the pairing window or tray menu.

---

## 5. Native Desktop Features

* **Windows 11 Mica & Dark Mode:** Uses Windows 11 Desktop Window Manager (DWM) APIs for an immersive dark titlebar and Fluent Design Mica backdrop blur.
* **Native Clipboard:** Enhanced table cell copying and row copying (as formatted JSON or TSV) integrated directly with Win32 clipboard APIs.
* **Window Memory:** Every window automatically remembers its screen coordinates, monitor, width, height, and maximized state across restarts (`%APPDATA%\DrizzleGateway\window.json`).

---

## 6. Data Storage & Paths

All application data is kept locally on your machine:

| Path | Description |
| :--- | :--- |
| `%APPDATA%\DrizzleGateway\data\store.json` | Database credentials and connection slots |
| `%APPDATA%\DrizzleGateway\data\mobile_state.json` | Mobile proxy port, IPs, and state |
| `%APPDATA%\DrizzleGateway\data\mobile_token.txt` | Secure authorization token for mobile pairing |
| `%APPDATA%\DrizzleGateway\window.json` | Window geometry, monitor index, and maximized state |
| `%APPDATA%\DrizzleGateway\webview2\` | WebView2 cache, query history, and table tab state |

---

## 7. Troubleshooting & FAQ

### "This app cannot be installed" on Android Chrome
* If connecting via an HTTP IP, Android Chrome blocks WebAPKs for security.
* Use the **Tailscale (Anywhere)** tab — Drizzle Gateway will auto-activate **trusted HTTPS**, which allows full standalone PWA installation.
* Alternatively, open the link in **Samsung Internet** (on Samsung phones) and tap **Add page to $\rightarrow$ App screen**.

### Exiting Leaves Orphaned Processes
* Drizzle Gateway incorporates Windows Job Objects and multi-pass tree termination. Simply right-click the tray icon and select **Quit Drizzle Gateway** to terminate all instances and child processes cleanly.

### Port Already in Use
* If port 4984 is taken, Drizzle Gateway automatically scans and binds the next available port (up to 4999).

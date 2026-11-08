# ⚡ Star Citizen Log Watcher: Quickstart Guide

This guide will help you run the **Star Citizen Forensic Black Box** on your machine in under 2 minutes.

---

## 🛠️ Step 1: Get the Executable

### Option A: You Were Sent the Pre-Compiled Binary (`StarCitizenLogWatcher.exe`)
No installation or runtime required! Put `StarCitizenLogWatcher.exe` anywhere on your computer (e.g. Desktop or a tools folder).

### Option B: Building from Source (Requires Go)
If you have Go installed:
```powershell
go build -o StarCitizenLogWatcher.exe .
```

---

## 📍 Where Are Star Citizen Logs Located?

By default, Star Citizen saves its logs here:
* **LIVE Environment:**  
  `C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\Game.log`
* **PTU Environment:**  
  `C:\Program Files\Roberts Space Industries\StarCitizen\PTU\Game.log`
* **Historical Backups (Past Sessions):**  
  `C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\logbackups\`

*(If installed on a secondary drive like `D:` or `E:`, update the drive letter accordingly).*

---

## 🎮 Step 2: Choose Your Workflow

Open **PowerShell** or **Command Prompt** in the folder containing `StarCitizenLogWatcher.exe` and pick one of the three workflows:

### Workflow 1: Instant Health Check (Check Last Session)
Want to know if your last session had ghost contract corruption or Subsumption failures?

```powershell
.\StarCitizenLogWatcher.exe -file "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\Game.log"
```
* **Output:** Prints an instant diagnosis to your screen and generates `Forensic_Report.md`.

---

### Workflow 2: In-Flight Recorder (Run While Playing)
Launch this right before or right after opening the RSI Launcher:

```powershell
.\StarCitizenLogWatcher.exe -file "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\Game.log" -watch
```
* **Live Dashboard:** Displays current session phase, DGS health, and active mission tracking in real time.
* **Yellow Alert Flash:** Flashes yellow whenever a contract event or Subsumption error occurs.
* **To Stop:** Press `Ctrl+C` when you exit the game to finalize your report.

---

### Workflow 3: Historical Deep Dive (Scan All Past Logs)
Want to analyze your entire history of logs to find out how far back ghost contracts date?

```powershell
.\StarCitizenLogWatcher.exe -dir "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\logbackups" -output "My_History_Report.md"
```
* Automatically sorts all past logs from oldest to newest.
* Connects old completed contracts to subsequent boot failures.

---

## 📊 Step 3: Understanding Your Results

### What the Terminal Output Means:
* **`Client Subsumption AI: [NOMINAL]` (Green):**  
  Your client AI initialized cleanly. No ghost contracts are disrupting your local game engine.
* **`Client Subsumption AI: [CRITICAL] FAILED ON STARTUP` (Red):**  
  ⚠️ **You are "Ephemeral"!** Your client failed to initialize Subsumption tasks during application launch because an un-cleared contract from a prior session persists on your account ledger. Missions and proximity markers will likely fail to spawn in this session.
* **`DGS System Health: X RPC Failures Detected`:**  
  Server communications dropped RPC registrations (often precedes a 30K or desync).

---

## 📁 Where Is My Full Report?

By default, the detailed report is saved as **`Forensic_Report.md`** (or the name passed to `-output`).

You can open it in **VS Code**, **Notepad**, or preview it in markdown viewers.

### Submitting to CIG Issue Council:
The generated report has a dedicated **"The Smoking Gun"** causality section. Copy and paste that table directly into your CIG Issue Council bug report to give developers the exact `MissionID`, timestamp, and error GUID proving the bug!


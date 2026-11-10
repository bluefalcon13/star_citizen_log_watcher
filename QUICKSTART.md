# ⚡ Star Citizen Log Watcher: Quickstart Guide

This guide will help you set up and run **Star Citizen Log Watcher** in under 2 minutes to locate, translate, and isolate events in your `Game.log` files.

---

## 🛠️ Step 1: Get the Executable

### Option A: Pre-Compiled Binary (`StarCitizenLogWatcher.exe`)
No installation required! Place `StarCitizenLogWatcher.exe` in any folder on your computer.

### Option B: Build from Source (Requires Go)
```powershell
go build -o StarCitizenLogWatcher.exe .
```

---

## 📍 Where Are Star Citizen Logs Located?

By default, Star Citizen saves its logs here:
* **Current Session (`LIVE`):**  
  `C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\Game.log`
* **Current Session (`PTU`):**  
  `C:\Program Files\Roberts Space Industries\StarCitizen\PTU\Game.log`
* **Past Sessions (`logbackups`):**  
  `C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\logbackups\`

*(If installed on another drive like `D:` or `E:`, change the drive letter).*

---

## 🎮 Step 2: Pick a Workflow

Open **PowerShell** or **Command Prompt** in the folder containing `StarCitizenLogWatcher.exe`:

### Workflow 1: Analyze a Log File (Static Mode)
Scan your last session (or any backup log) and generate a clean Markdown summary (`Forensic_Report.md`):

```powershell
.\StarCitizenLogWatcher.exe -file "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\Game.log"
```
To save to a specific report filename (useful when comparing two logs side by side):
```powershell
.\StarCitizenLogWatcher.exe -file "path\to\backup.log" -output "My_Session_Report.md"
```

---

### Workflow 2: Live Monitor While Playing (Watch Mode)
Run this before or during gameplay to watch events appear in real time:

```powershell
.\StarCitizenLogWatcher.exe -file "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\Game.log" -watch
```
* **Live Dashboard:** Shows your current session state (`Main Menu`, `Joining Server`, `Gameplay`), active contracts, and plain-English log events.
* **Yellow Flash:** Flashes yellow whenever a tracked log pattern is detected.
* **To Stop:** Press `Ctrl+C` to exit and save your Markdown report.

---

### Workflow 3: Scan a Folder of Historical Logs (Directory Mode)
Analyze an entire folder of `.log` files in chronological order (oldest to newest):

```powershell
.\StarCitizenLogWatcher.exe -dir "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\logbackups" -output "History_Report.md"
```

---

### Workflow 4: Scrape All Accepted Contracts & Outcomes (`-contracts`)
Quickly list every contract you accepted in a log file (or across your entire `logbackups` folder), including the contract title, internal blueprint, outcome (`Completed`, `Failed`, `Abandoned`, `Stalled`), and any anomalies:

```powershell
.\StarCitizenLogWatcher.exe -contracts -dir "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\logbackups" -output "Contracts_Report.md"
```

---

## 🔧 Step 3: Customize What You Watch (`filters.json`)

The first time you run `StarCitizenLogWatcher.exe`, it creates **`filters.json`** in the same folder. You can open `filters.json` in Notepad or VS Code to customize what the tool looks for—no recompiling needed:

* **`"pattern"`**: The regular expression to search for in `Game.log`.
* **`"meaning"`**: The plain-English message displayed when that line is found.
* **`"enabled"`**: Set to `false` to hide noisy log events, or `true` to watch them.
* **`"active_investigation"`**: Set to `true` on any specific log pattern you are actively hunting. Any matches will be highlighted as **Active Investigation Incidents** in both the terminal and the top of your Markdown report!

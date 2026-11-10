# 🛰️ Star Citizen Log Watcher

A lightweight, high-performance log analyzer and real-time flight recorder for **Star Citizen**. Built in Go with an asynchronous concurrency pipeline, `StarCitizenLogWatcher` helps you **locate, translate, and isolate specific log messages**—either live while playing or through static analysis of saved `Game.log` files—without digging through hundreds of thousands of lines of raw engine output.

---

## 💡 What This Tool Helps You Do

Star Citizen's `Game.log` easily grows to 50,000–200,000+ lines per session, burying important events under repetitive engine, inventory, and UI spam. `StarCitizenLogWatcher` turns raw logs into readable, actionable timelines:

1. **Locate & Translate Specific Log Events (`filters.json`)**:
   * Define regular expression filters in a simple JSON file (`filters.json`) and assign each one a **plain-English meaning**.
   * Add new filters, tweak descriptions, or disable noisy rules (`"enabled": false`) in any text editor—**no recompiling required**.
2. **Isolate Bugs & Hypotheses (`active_investigation`)**:
   * Flag any filter with `"active_investigation": true` to elevate matching lines into high-visibility **Active Investigation Alerts** in both the live terminal dashboard and the generated Markdown report.
   * Perfect for A/B testing logs (e.g., comparing a known-good session against a bugged session, or comparing two accounts) to separate real root-cause errors from baseline game noise.
3. **Track Session & Contract Context Automatically**:
   * Automatically tracks where you are in the session lifecycle (`Application Launch` $\rightarrow$ `Main Menu` $\rightarrow$ `Joining Server` $\rightarrow$ `Gameplay`) and which server session (`#1`, `#2`, etc.) an event occurred in.
   * Extracts `MissionId` and `GUID` values automatically so every event is tied to what you were doing in-game.
4. **Watch Live or Analyze Past Logs**:
   * **Live Watch Mode (`-watch`)**: Run alongside the game on a second monitor. The terminal dashboard flashes yellow whenever a tracked event occurs in real time.
   * **Static Mode (`-file`)**: Parse any existing `Game.log` or backup log in milliseconds and generate a clean Markdown report (`Forensic_Report.md`).
   * **Batch Directory Mode (`-dir`)**: Scan an entire `logbackups` folder chronologically from oldest to newest.

> 📄 **Example Investigation:** See [ghost_investigation.md](ghost_investigation.md) for a real-world walkthrough of how this tool was used to compare logs across accounts and historical dates to isolate broken mission proximity triggers.

---

## 🚀 Getting Started

### Option A: Pre-Compiled Binary
No installation or runtime required. Place `StarCitizenLogWatcher.exe` in any folder and run it from PowerShell or Command Prompt. On first run, it automatically generates a default `filters.json` next to the executable.

### Option B: Build from Source
Requires **Go 1.20+** (Windows, Linux, or macOS):
```powershell
go build -o StarCitizenLogWatcher.exe .
```

---

## 💻 Usage & Workflows

### 1. Static Analysis (Inspect a Single `Game.log`)
Extract all matching events from a log file into a structured Markdown report and terminal summary:

```powershell
# Analyze your current LIVE Game.log
.\StarCitizenLogWatcher.exe -file "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\Game.log"

# Analyze a specific historical backup log and save to a custom report file
.\StarCitizenLogWatcher.exe -file "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\logbackups\Game Build(...).log" -output "Session_Report.md"
```

### 2. Side-by-Side A/B Log Comparison
When troubleshooting a bug, run the tool against a log where everything worked and a log where the issue occurred, then diff the resulting Markdown reports:

```powershell
.\StarCitizenLogWatcher.exe -file "path\to\good_session.log" -output "reports\good.md"
.\StarCitizenLogWatcher.exe -file "path\to\bad_session.log" -output "reports\bad.md"
```

### 3. Live Watch Mode (Real-Time Dashboard)
Monitor `Game.log` in real time while playing Star Citizen:

```powershell
.\StarCitizenLogWatcher.exe -file "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\Game.log" -watch
```
* Displays live line counts, current session phase (`Main Menu`, `Joining Server`, `Gameplay`), active contract status, recent state changes, and plain-English log entries.
* Flashes **Yellow** for 2 seconds whenever a new matching event is logged.
* Press `Ctrl+C` to stop watching and write the final report to disk.

### 4. Directory Batch Mode (Scan Entire Log History)
Process every `.log` file in a folder in chronological order (oldest to newest):

```powershell
.\StarCitizenLogWatcher.exe -dir "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\logbackups" -output "Full_History_Report.md"
```

### 5. Contract Ledger Mode (`-contracts`)
Scrape every accepted contract across a single log or an entire directory of historical logs, along with its human-readable name, internal blueprint template, session timestamp, outcome (`Completed`, `Failed`, `Abandoned`, `Stalled (Hyperion Bug)`, `Unfinished`), and any mid-mission anomalies:

```powershell
# Scrape contracts from a single log file
.\StarCitizenLogWatcher.exe -contracts -file "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\Game.log"

# Scrape contracts across your entire logbackups history
.\StarCitizenLogWatcher.exe -contracts -dir "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\logbackups" -output "Contract_Ledger.md"
```

### CLI Flags Reference

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-file` | string | `""` | Path to a target `.log` file (or a folder of logs). |
| `-dir` | string | `""` | Directory containing `.log` files to batch process chronologically. |
| `-input` | string | `""` | Alias for `-file`. |
| `-contracts` | bool | `false` | Scrape and list all accepted contracts, blueprints, and their outcomes. |
| `-watch` | bool | `false` | Enable live tailing dashboard for active gameplay sessions. |
| `-output`| string | `Forensic_Report.md` | Output path for the generated Markdown report. |
| `-filters`| string | `filters.json` | Path to the JSON filter configuration file (auto-created if missing). |
| `-verbose`| bool | `false` | Print verbose progress details during ingestion. |

---

## 🛠️ Customizing Filters (`filters.json`)

All log matching is controlled by `filters.json`. You can open `filters.json` in Notepad, VS Code, or any text editor to add your own patterns or adjust existing ones:

```json
[
  {
    "name": "Mission Accepted",
    "enabled": true,
    "event_type": "Mission Accepted",
    "severity": "INFO",
    "pattern": "Contract Accepted: .* MissionId: \\[([a-f0-9\\-]{36})\\]",
    "extract_type": "mission_id",
    "target_state": "Gameplay",
    "meaning": "Player accepted a contract",
    "active_investigation": false
  },
  {
    "name": "Custom Log Pattern",
    "enabled": true,
    "event_type": "Custom Event",
    "severity": "WARNING",
    "pattern": "<MyEngineTag> Something happened with entity \\[(\\d+)\\]",
    "extract_type": "none",
    "meaning": "Custom engine event occurred on entity",
    "active_investigation": true
  }
]
```

### Filter Fields Explained
* **`name`**: Descriptive name for the filter rule.
* **`enabled`**: `true` or `false`. Set to `false` to mute a rule without deleting it.
* **`event_type`**: Short category tag displayed in the TUI and report tables.
* **`severity`**: `NOTICE`, `INFO`, `WARNING`, `ERROR`, or `CRITICAL`.
* **`pattern`**: Regular expression matched against each raw log line. Add a capture group `(...)` if you want to extract a specific ID, name, or value from the line.
* **`extract_type`**: Controls how the first regex capture group `(...)` is used:
  * `"mission_id"` — Extracts a contract `MissionId` and updates the active contract tracker.
  * `"error_guid"` — Extracts an `Error GUID` into the report's GUID column.
  * `"status"` / `"result"` — Formats contract status or outcome strings.
  * `"none"` — Appends the captured value in parentheses to the end of your `meaning` text.
* **`target_state`** *(optional)*: Updates the macro session state when matched (`"Main Menu"`, `"Joining Server"`, or `"Gameplay"`).
* **`meaning`**: Plain-English translation displayed in the terminal dashboard and Markdown report.
* **`active_investigation`**: Set to `true` when you want matching events highlighted in the **Active Investigation Incidents** alert section at the top of the report and terminal summary.

---

## 🏗️ Internal Architecture

The engine separates file I/O from regex processing and terminal rendering so it never slows down the game client:

```
                  ┌───────────────┐
                  │   Game.log    │
                  └───────┬───────┘
                          │ (StreamStatic / StreamWatch)
                          ▼
                  ┌───────────────┐
                  │   Ingestor    │  Producer (Managed Goroutine)
                  └───────┬───────┘
                          │ Buffered Raw Line Channel (1000 cap)
                          ▼
                  ┌───────────────┐
                  │   Processor   │  Central Brain / Event Loop
                  │  (Filter Reg) │  Evaluates rules from filters.json
                  └───────┬───────┘
                          │ Buffered Event Channel (256 cap)
             ┌────────────┼────────────┐
             ▼            ▼            ▼
      ┌────────────┐┌───────────┐┌───────────┐
      │   State    ││ Markdown  ││    TUI    │
      │  Manager   ││ Recorder  ││ Dashboard │
      │(sync.RWMutex││(File I/O) ││(Observer) │
      └────────────┘└───────────┘└───────────┘
```

---

## 🧪 Running Tests

To run the unit and integration test suite across all packages:

```powershell
go test -v ./...
```

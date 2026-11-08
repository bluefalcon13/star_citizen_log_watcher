# 🛰️ Star Citizen Log Watcher: Forensic "Black Box" Flight Recorder

A lightweight, high-performance forensic log watcher and real-time flight recorder for **Star Citizen**. Built in Go with an asynchronous, decoupled concurrency pipeline to monitor, diagnose, and capture undeniable forensic proof of account-side state desync and Subsumption service failures.

---

## 🎯 The Mission: Catching the "Smoking Gun"

### The Problem: Latent State Corruption & Ephemeral Players
In Star Citizen, a critical bug occurs when server-side ledgers fail to clear or reconcile completed or active contracts. This "ghost state" persists across sessions. Upon reconnecting, the client's **Subsumption service** (responsible for mission logic, AI behaviors, and proximity triggers) fails to map the archetype or GUID. 

This causes the player to become **"Ephemeral"**—physically inside the game world, but invisible to mission triggers, objective markers, and AI spawns.

### The Solution: Forensic Flight Recorder & Cross-Launch Causal Bridge
The Log Watcher acts as an automated "Black Box" flight recorder that bridges the temporal gap between cause and symptom:
1. **Application Launch Subsumption Flag:** Immediately identifies when Subsumption AI fails during initial engine boot, flagging the entire session as `Tainted / Ephemeral` before the player even joins a server.
2. **Persistent Historical Ledger (`.sc_latent_state.json`):** Tracks completed/active contracts from **Session N**. When Subsumption throws startup errors in **Session N+1**, it directly correlates the failed GUID against the prior session's contracts to prove the causal link.
3. **Macro Session Lifecycle Awareness:** Categorizes logs into `Application Launch` $\rightarrow$ `Main Menu` $\rightarrow$ `Joining Server` $\rightarrow$ `Gameplay`, handling server disconnects and re-launches across different DGS instances.
4. **Condensed Forensic Dossiers:** Produces structured Markdown forensic reports with executive summaries, startup diagnostics, and chronological timelines ready for CIG Issue Council reports.

---

## 🏗️ Architecture: Asynchronous Pipeline

The application decouples the fast log-reading path from UI rendering and disk I/O, guaranteeing zero impact on gameplay frame rates or system performance:

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
                  │  (Filter Reg) │
                  └───────┬───────┘
                          │ Buffered Event Channel (256 cap)
             ┌────────────┼────────────┐
             ▼            ▼            ▼
      ┌────────────┐┌───────────┐┌───────────┐
      │   State    ││ Forensic  ││    TUI    │
      │  Manager   ││ Recorder  ││ Dashboard │
      │(sync.RWMutex││(File I/O) ││(Observer) │
      └────────────┘└───────────┘└───────────┘
```

* **Ingestor (Producer):** Reads log files in one-shot static mode or tails live sessions with truncate/rotation resilience.
* **Processor (Brain):** Dispatches raw lines through an Open-Closed **Filter Registry** of compiled regex signatures without hardcoded conditional switches.
* **State Manager (Telemetry Store):** Thread-safe metrics store maintaining active mission states, live error tallies, and desync chain causality checks.
* **Recorder (Consumer):** Aggregates chronological events and compiles the markdown forensic report (`Forensic_Report.md`).
* **Dashboard (Observer):** Renders a real-time terminal display featuring visual yellow highlights on recent state changes.

---

## 🚀 Getting Started

### Prerequisites
* **Go 1.20+** (tested and compiled on Go 1.27)
* Windows, Linux, or macOS

### Installation & Build

Clone the repository and build the single, zero-dependency executable:

```powershell
go build -o StarCitizenLogWatcher.exe .
```

---

## 💻 Usage

### 1. Static Mode (One-Shot Forensic Analysis)
Analyze an existing `.log` file from a previous session to extract its event timeline and verify if a desync occurred:

```powershell
# Basic single file analysis
.\StarCitizenLogWatcher.exe -file "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\Game.log"

# Custom output report path and verbose logging
.\StarCitizenLogWatcher.exe -file "Game.log" -output "Desync_Investigation.md" -verbose
```

### 2. Directory Batch Mode (Multi-Session Chronological Analysis)
You can feed an entire folder containing multiple `.log` files (e.g. historical sessions, rotated logs):

```powershell
# Process all .log files in a folder chronologically
.\StarCitizenLogWatcher.exe -dir "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE"

# Or pass the folder directly to -file (automatically detected)
.\StarCitizenLogWatcher.exe -file "example_logs" -output "Combined_Report.md"
```

* Discovers all `.log` files in the directory.
* Sorts them chronologically by file modification time so that older sessions (Session $N$) run before newer sessions (Session $N+1$).
* Carries forward the persistent contract ledger so Subsumption errors in Session $N+1$'s boot immediately link to the contract completed in Session $N$!

### 3. Live Watch Mode (In-Flight Recorder)
Run alongside an active game session to monitor mission state and system health live:

```powershell
.\StarCitizenLogWatcher.exe -file "C:\Program Files\Roberts Space Industries\StarCitizen\LIVE\Game.log" -watch
```

* UI displays live line ingestion counts, DGS health, active mission tracker, and errors.
* Elements highlight with a **Yellow Flash** for 2 seconds whenever a new event is captured.
* Press `Ctrl+C` to cleanly terminate the session and finalize the forensic report.

### CLI Flags Reference

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-file` | string | `""` | Path to target log file (`Game.log`), or a directory of logs. |
| `-dir` | string | `""` | Directory containing `.log` files to batch process chronologically. |
| `-input` | string | `""` | Legacy alias for `-file`. |
| `-watch` | bool | `false` | Enable real-time tailing mode during active gameplay. |
| `-output`| string | `Forensic_Report.md` | Destination path for the generated forensic report. |
| `-verbose`| bool | `false` | Enables verbose diagnostic output to stdout. |

---

## 🔍 Monitored Pattern Signatures

All patterns defined in `docs/draft_contract_filter.md` and `docs/log_patterns.md` are actively detected:

| Event Signature | Category | Severity | Forensic Significance |
| :--- | :--- | :--- | :--- |
| **Mission Accepted** | Contract Lifecycle | `INFO` | Tracks mission initiation and valid `MissionId` GUID. |
| **Objective Spawned** | World Manifestation | `INFO` | Verifies physical phase marker (`CLocalMissionPhaseMarker`). Missing this indicates an ephemeral desync. |
| **Mission Completed** | Contract Lifecycle | `INFO` | Marks clean completion of mission parameters. |
| **Mission Failed** | Contract Lifecycle | `WARNING` | Marks clean failure of mission logic. |
| **Mission Abandoned**| Contract Lifecycle | `WARNING` | Marks player-initiated mission cancellation. |
| **Subsumption Error**| AI Subsumption | `ERROR` | Primary indicator of AI/Archetype mismatch (`Subsumption::ErrorReporter`). |
| **Subsumption Mismatch**| AI Subsumption | `ERROR` | Catches missing tasks, variables, and dependencies. |
| **DGS Failure** | Network / Services | `CRITICAL` | Distributed Game Service RPC registration failure (`RPC.DGS.*`). |
| **Contract Init** | System Lifecycle | `NOTICE` | Verifies contract generator initialization (`CContractGenerator`). |
| **Contract Status** | System Lifecycle | `INFO` | Validates reported contract generator status transitions. |
| **Mission Result** | State Validation | `INFO` | Validates engine-reported mission outcomes (`Success`/`Failure`). |

---

## 📊 Sample Forensic Report Output

When analysis completes, the recorder generates a clean Markdown report:

```markdown
# 🕵️ Star Citizen Forensic Black Box Report

**Report Generated:** 2026-11-08T21:52:29Z
**Total Lines Ingested:** 152,430
**Total Filtered Events:** 18

## 📊 [Summary]
| Event Type | Count |
| :--- | :--- |
| **Mission Accepted** | 1 |
| **Subsumption Error** | 1 |
| **DGS Failure** | 1 |
| **Contract Status** | 1 |
| **Mission Completed** | 1 |

## 🔍 [Forensic Causality Analysis: The Smoking Gun]
⚠️ **DESYNC SIGNATURES DETECTED:** 1 latent state collision(s) identified.

| Incident # | Timestamp | Mission ID | Diagnostic Finding |
| :--- | :--- | :--- | :--- |
| 1 | `2026-10-21T05:02:15.500Z` | `40a7a5ca-59da-11f1-55d5-d3a0356e3f93` | Smoking Gun: Subsumption failure occurred before mission objective marker manifested (Ephemeral Player bug) |

## ⏱️ [Chronological Timeline]
| Timestamp | Severity | Event Type | Mission ID | Error GUID | Details |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 2026-10-21T05:02:14.116Z | `INFO` | Mission Accepted | `40a7a5ca-...` | `—` | Contract Accepted: Bounty Hunt MissionId: [40a7a5ca-...] |
| 2026-10-21T05:02:15.500Z | `ERROR` | Subsumption Error | `—` | `40a7a5ca-...` | Failed to create Subsumption variable 'List' (GUID:40a7a5ca-...) |
```

---

## 🧪 Testing & Verification

The codebase includes comprehensive test suites across every package adhering strictly to `docs/standards.md`:

```powershell
# Run all unit tests and integration tests
go test -v ./...
```

Tests cover:
* Pluggable regex pattern matching and metadata extraction against real Star Citizen log formats.
* Ingestor static EOF processing and live watch context cancellation.
* Processor channel routing and state manager integration.
* Thread-safe concurrency stress tests for the State Manager.
* Forensic markdown report rendering and timeline ordering.
* End-to-end pipeline execution.

---

## 📜 Go Coding Standards Compliance

Built to adhere strictly to the engineering rules outlined in `docs/standards.md`:
* **Zero Panics:** Robust error propagation and context wrapping (`%w`).
* **Dependency Injection:** Constructors (`NewProcessor`, `NewManager`, `NewRecorder`, `NewIngestor`) isolate configuration and dependencies.
* **Managed Goroutines:** Every background process is tied to `sync.WaitGroup` and `context.Context` lifecycle handling.
* **Channel Ownership:** Strict producer-closes-channel discipline with justified buffering.
* **No Global State:** Pure package isolation with encapsulated data structures.


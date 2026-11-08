# 📄 Running Design Document: Star Citizen Log Watcher
**Status:** *Finalized Design - Ready for Implementation Planning*  
**Objective:** *High-fidelity, real-time forensic monitoring and recording of Star Citizen client logs to identify and prove account-side state corruption via Subsumption service failures.*

## 1. Executive Summary & Vision
**The Problem:** A critical "Latent State Corruption" bug exists where contracts are not cleared from the server-side ledger upon completion. This "ghost" state persists on the player's account. Upon the next login, the client-side Subsumption service (responsible for AI, proximity triggers, and mission logic) fails to reconcile this existing contract, causing the player to become "ephemeral"—effectively invisible to mission-critical triggers, waypoints, and AI spawns.

**The Vision:** A "Black Box" flight recorder that runs alongside the Star Citizen client. This tool will provide real-time visual awareness of mission and system health via a CLI dashboard, while simultaneously performing high-speed forensic recording. Its ultimate purpose is to provide developers with an undeniable "Smoking Gun" report that links a specific mission completion to a subsequent Subsumption failure upon account re-login.

## 2. Core Requirements

### 2.1 Functional Requirements
* **Automated Session Hooking:** Launch the RSI Launcher and begin non-intrusive, real-time tailing of `Game.log` once the game process is detected.
* **Real-Time Telemetry (TUI Dashboard):**
    * A live Command Line Interface displaying:
        * **Contract Tracker:** Real-time status of the last mission (Success/Failure/Completed).
        * **System Health:** DGS (Distributed Game Service) connectivity and restart events.
        * **Error Counter:** Cumulative Subsumption/UUID errors.
    * **Visual Awareness:** UI elements must "Flash" (highlighted in Yellow) for a short duration when a new event is detected, providing immediate feedback for recent state changes.
* **Forensic Black Box Recording:**
    * Continuous, condensed logging of high-value events to a dedicated `Forensic_Report.log`.
    * **Targeted Events:** Contract completions, DGS disruptions, and all `Subsumption::ErrorReporter` entries.
    * **The Desync Trigger:** Detection of Subsumption errors occurring during the session's initial startup window.

### 2.2 Non-Functional Requirements
* **High Throughput/Low Overhead:** Must use a non-blocking, asynchronous architecture to ensure the "hot path" of log reading is never delayed by UI rendering or Disk I/O.
* **Resilience:** Must handle file rotation (when the game overwrites `Game.log`) and manage the "Latent State" across different game sessions.
* **Single-Binary Portability:** A compiled, zero-dependency executable (Go/Rust) for easy distribution to testers.
* **Modular Expandability:** The pattern-matching engine must be designed to allow the rapid addition of new regex filters and event types as new bug signatures are discovered, without requiring a refactor of the core concurrency pipeline.

## 3. Pattern Specification
The following regex patterns are the primary intelligence for the Event Loop.

| Category | Regex Pattern | Log Example |
| :--- | :--- | :--- |
| **DGS Failure** | `!!! Failed to register RPC \[RPC\.DGS\..*\]` | `!!! Failed to register RPC [RPC.DGS.CommArray.Manage]` |
| **Subsumption Error** | `\[Error\] <\[Subsumption::ErrorReporter\]> .* \(GUID:[a-f0-9\-]{36}\) .* \[Team_CoreAI\]\[Subsumption\]` | `[Error] <[Subsumption::ErrorReporter]> ... (GUID:40a7a5ca...) ... [Team_CoreAI][Subsumption]` |
| **Subsumption Mismatch**| `Failed to create Subsumption (task\|variable\|dependency) .* associated with the guid '([a-f0-9\-]{36})'` | `...Failed to create Subsumption task because the Archetype associated with the guid 'e0a668ab...' is not present...` |
| **Contract State** | `\[CContractGenerator::CContractGenerator\] .* (completed\|failed\|closed)` | `[Notice] <CContractGenerator::CContractGenerator> ... [Missions] (Status: completed)` |
| **Mission Result** | `(Contract\|Mission) .* (result\|outcome) is (Success\|Failure\|Completed\|Failed)` | `Mission result is Success` |

## 4. System Architecture: The "Asynchronous Pipeline"

### 4.1 Component Breakdown
1.  **Ingestor (Producer):** A `go routine` that tails `Game.log`. It pushes raw strings into a buffered `RawLineChannel`.
2.  **Event Loop (Processor):** The central "Brain." It runs a **Filter Registry** (a collection of pluggable regex patterns) against incoming lines. It dispatches results to the Recorder, the State Manager, and the UI.
3.  **State Manager (Data Store):** A centralized `struct` protected by a `sync.RWMutex`. It maintains the current live counters and timestamps for the UI.
4.  **Recorder (Consumer):** A `go routine` that listens to an `EventChannel` and writes the forensic, condensed timeline to `Forensic_Report.log`.
5.  **CLI App (Observer):** A `go routine` running a TUI (Terminal User Interface). It polls the State Manager to update the dashboard and manages the "Yellow Flash" timer.

### 4.2 The "Filter Registry" (Expandability Pattern)
To satisfy the requirement for expandability, the Event Loop will not use a hardcoded `switch` statement for regex matching. Instead, it will iterate through a **Registry of Filter Modules**.
* Each **Filter Module** defines:
    * A **Regex Pattern**.
    * An **Event Type** (e.g., `EventSubsumptionError`).
    * A **Priority/Severity** level.
* Adding a new filter involves simply adding a new entry to the Registry slice, making the system "Open-Closed" (Open for extension, closed for modification of the core engine).

## 5. Technical Stack & Constraints
* **Primary Language:** **Go (Golang)** (for goroutines, channels, and single-binary builds).
* **UI Library:** TUI (e.g., `bubbletea` or `tview`).
* **Strict Constraints:** **No Python.** No heavy external dependencies. No blocking the I/O thread.
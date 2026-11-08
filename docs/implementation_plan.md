# 🗺️ Implementation Roadmap: Star Citizen Log Watcher
**Status:** *Approved - Ready for Execution*  
**Primary Goal:** *Build a high-performance, asynchronous forensic "Black Box" for Star Citizen logs using Go.*

## 🏗️ Core Architecture Principles
* **Concurrency Model:** Producer (Ingestor) $\rightarrow$ Channel $\rightarrow$ Consumer (Event Loop) $\rightarrow$ Channel $\rightarrow$ Observers (Recorder/TUI).
* **Decoupling:** The "Hot Path" (Log Reading) must never wait for "Slow Paths" (Disk I/O or UI Rendering).
* **Data-Driven:** All detection logic is handled by a pluggable `FilterRegistry` to allow for rapid expansion as new bug signatures emerge.

---

## 🚀 Execution Phases

### Phase 1: The Core Engine (The "Brain")
*Goal: Build the processing logic and verify it works against static files before introducing real-time complexity.*

* **1.1 Scaffolding & Types:**
    * Initialize Go module.
    * Define `Event` and `Filter` structures.
    * Define `EventType` enums (Contract, DGS, Subsumption, etc.).
* **1.2 The Event Loop (The "Processor"):**
    * Implement the central loop that consumes raw lines and runs the `FilterRegistry`.
    * Implement the `FilterRegistry` to allow adding new regex patterns without refactoring the engine.
* **1.3 Static Mode (The "Validator"):**
    * Implement a `StaticReader` that performs a one-shot read of a provided `.log` file.
    * **Success Criteria:** Feeding an existing log file into the engine produces the correct categorized event counts and a condensed forensic summary.
* **1.4 CLI Argument Parser:**
    * Implement flags: `--file` (for static mode), `--watch` (for live mode), `--verbose` (for debugging).

### Phase 2: Real-Time Observability (The "Dashboard")
*Goal: Transform the data into a human-readable, live monitoring interface.*

* **2.1 The State Manager:**
    * Implement a thread-safe `struct` using `sync.RWMutex` to store live counters (Errors, Contracts, DGS status).
* **2.2 TUI Implementation (The "Monitor"):**
    * Implement a live dashboard (e.g., using `bubbletea` or `tview`).
    * **Visual Feedback:** Implement the "Yellow Flash" logic (color-coding text based on the time since the last event).
* **2.3 Live Ingestor (The "Watcher"):**
    * Implement a file-tailing mechanism that handles file growth and rotation.
    * Connect the Ingestor to the Event Loop via buffered channels.

### Phase 3: System Integration (The "Wrapper")
*Goal: Automate the workflow and make the tool "Plug-and-Play" for players.*

* **3.1 Automation & Orchestration:**
    * Implement RSI Launcher detection.
    * Implement process-waiting logic (ensuring the tool starts reading *after* the game has initialized).
* **3.2 Environment Configuration:**
    * Implement `.env` file loading for configurable paths (e.g., `LOG_PATH`, `OUTPUT_DIR`).
    * Provide `.env.example` for users.

### Phase 4: Deployment & Validation
*Goal: Final testing and "Production" readiness.*

* **4.1 Live Stress Testing:**
    * Run the tool during active gameplay to ensure zero impact on CPU/Memory.
* **4.2 Final Verification:**
    * Run a full session to capture a "Desync Event" and verify the Forensic Report provides the "Smoking Gun."
* **4.3 Production Release:**
    * Compile into a single, lightweight binary.
    * "YOLO" (Final deployment to field testing).

---

## 🛠️ Technical Stack Summary
| Component | Technology |
| :--- | :--- |
| **Language** | **Go (Golang)** |
| **UI** | TUI Library (`bubbletea` / `tview`) |
| **Pattern Matching** | Standard `regexp` package |
| **Concurrency** | Goroutines, Channels, `sync.RWMutex` |
| **Environment** | `.env` via `godotenv` (or similar) |
| **Constraint** | **Strictly No Python** |
# 🛠️ Agent Instructions: Forensic Incremental Analysis

**Goal:** Perform a deep-dive forensic reconstruction of massive `.log` files that exceed the agent's immediate context window.

## 🧠 The "Incremental Reconstruction" Algorithm

When tasked with analyzing a log file, the Agent must not attempt to read the entire file at once. Instead, follow this iterative loop:

### 1. The Chunking Phase (Data Acquisition)
* **Divide and Conquer:** Access the file in manageable chunks (e.s., 500–1000 lines at a time) to respect the token limit.
* **Maintain Continuity:** Ensure chunk boundaries do not split a single log line in half.
* **Context Awareness:** For every chunk, record the start and end `Timestamp` to maintain a continuous chronological timeline.

### 2. The Extraction Phase (Pattern Application)
* **Filter Application:** Apply the regex patterns defined in `docs/draft_contract_filter.md` to the current chunk.
* **Categorization:** For every match, extract the `timestamp`, `event_type`, `mission_id`, and `message_raw`.
* **State Tracking:** Monitor for "Transition Patterns" (e.g., a `Mission Accepted` in one chunk followed by a `Subsumption Error` in a subsequent chunk).

### 3. The Iterative Update Phase (The "Working Draft")
* **Maintain a Running Draft:** As each chunk is processed, update a persistent draft of the analysis.
* **Draft Structure:**
    * **[Summary]:** Aggregated counts of event types found so far.
    * **[Timeline]:** A chronological list of extracted events.
    * **[Pending Threads]:** "Open" events (e.g., a mission started but not yet completed/failed) that require the next chunk to resolve.

### 4. The Completion Phase (Final Synthesis)
Once the end of the file is reached:
* **Consolidate:** Merge the running timeline into a final, structured `Forensic_Report.md`.
* **Analyze Causality:** Compare the total event history against the "Desync Chain" (The Smoking Gun) to identify if the account-side corruption is present.

---

## 🚫 Prohibited Actions (Strict Enforcement)
* **NO Hallucinations:** If a log chunk contains no matches, do not "guess" or invent events. Report: `[Chunk X]: No relevant signatures detected.`
* **NO Omissions:** Do not skip chunks to save time. Every line must be accounted for.
* **NO Large-File Attempts:** Never attempt a `read_file` call on a file that exceeds the token limit. Always request a chunked read or a specific line range.

---

## 🎯 Success Criteria
The analysis is successful only if the Agent can provide:
1. A complete, timestamped timeline of events.
2. A definitive statement on whether a "Mission-to-Subsumption" desync occurred.
3. The exact `MissionId` and `Timestamp` of the "Smoking Gun" event.
# 📑 Fingerprint Registry: Contract & Mission Events

**Version:** 1.5 (Unified: Precision + Forensic Context)  
**Purpose:** Definitive regex signatures and forensic rationale for identifying mission and contract state transitions. This is the "Intelligence Pack" for the Log Watcher.

## 🔍 Overview: The Forensic Mission
The goal is to detect **Latent State Corruption**. We are looking for a "Collision" where:
1. A mission is active in the client's "Happy Path."
2. A Subsumption error or DGS failure occurs.
3. The player becomes "Ephemeral" (mission triggers/waypoints fail to fire).

---

## 1. Mission Lifecycle Fingerprints (The "Happy Path")
These patterns track the successful lifecycle of a mission. We use these as a baseline to detect when a mission *should* be active but isn't.

| Event Type | Regex Pattern | Log Context | Forensic Significance |
| :--- | :--- | :--- | :--- |
| **Mission Accepted** | `Contract Accepted: .* MissionId: \[[a-f0-9\-]{36}\]` | `SHUDEvent_OnNotification` | Establishes the start of the mission lifecycle and the valid `MissionID`. |
| **Mission Completed** | `Contract Complete: .* MissionId: \[[a-f0-9\-]{36}\]` | `SHUDEvent_OnNotification` | Marks the intended end-state of a successful mission. |
| **Mission Failed** | `Contract Failed: .* MissionId: \[[a-f0-9\-]{36}\]` | `SHUDEvent_OnNotification` | Marks a clean failure of mission logic. |
| **Mission Abandoned** | `Contract Abandoned: .* MissionId: \[[a-f0-9\-]{36}\]` | `SHUDEvent_OnNotification` | Marks a user-initiated mission termination. |

## 2. World Manifestation Fingerprints
These verify that the game engine has actually "spawned" the mission into the physical world.

| Event Type | Regex Pattern | Log Context | Forensic Significance |
| :--- | :--- | :--- | :--- |
| **Objective Spawned** | `CLocalMissionPhaseMarker::CreateMarker.*missionId \[([a-f0-9\-]{36})\]` | `CLocalMissionPhaseMarker` | **CRITICAL:** If this fails to appear after a "Mission Accepted" event, the player is "Ephemeral." |
| **Contract Init** | `CContractGenerator::CContractGenerator` | `[Notice]` | Confirms the engine's contract generator has initialized for the session. |

## 3. System & Error Fingerprints (The "Collision" Signatures)
These identify the technical failures that disrupt the mission lifecycle.

| Event Type | Regex Pattern | Log Context | Forensic Significance |
| :--- | :--- | :--- | :--- |
| **DGS Failure** | `!!! Failed to register RPC \[RPC\.DGS\..*\]` | `[Notice] <[DGS_Context]>` | Indicates a network/service drop that may desync the session. |
| **Subsumption Error** | `\[Error\] <\[Subsumption::ErrorReporter\]> .* \(GUID:[a-f0-9\-]{36}\)` | `[Error] <[Subsumption::ErrorReporter]>` | **CRITICAL:** The primary indicator of AI/Task/Archetype mismatch. |
| **Subsumption Mismatch**| `Failed to create Subsumption (task\|variable\|dependency) .* associated with the guid '([a-f0-9\-]{36})'` | `[Error] <[Subsumption::ErrorReporter]>` | Specifically identifies archetype/resource missing errors. |

---

## 🛠️ Metadata Extraction Schema
To build a forensic timeline, the engine must extract and map the following fields for every match:

* **`timestamp`**: The ISO-8601 timestamp (e.g., `2026-10-21T05:02:14.116Z`).
* **`event_type`**: The specific category (e.g., `MissionCompleted`).
* **`mission_id`**: The 36-character GUID extracted from the `MissionId: [...]` block.
* **`error_guid`**: (If applicable) The specific Subsumption GUID that failed.
* **`message_raw`**: The full, original log line to preserve context for developers.

## 📈 Forensic Logic: The "Desync Chain"
The application should alert when the following sequence is detected within a short temporal window:
`[Mission Accepted]` $\rightarrow$ `[Objective Spawned (or missing)]` $\rightarrow$ `[Subsumption Error]` $\rightarrow$ `[Mission Status: Inactive/Missing]`
# 🕵️ Ghost State & Ephemeral Player Investigation Log

This document records the chronological progression of our forensic investigation into the **Star Citizen "Ephemeral Player" / Broken Proximity Trigger bug** on account `Bluefalcon2009`, demonstrating how iterative A/B log analysis isolated the true engine-level failure from baseline client/server noise.

---

## Phase 1: Initial Hypothesis — Startup `[Subsumption::ErrorReporter]` & DGS RPC Failures

### What We Initially Suspected
When inspecting raw `Game.log` files from broken gameplay sessions, two dramatic error signatures appeared immediately during `Application Launch`:
1. **`[Subsumption::ErrorReporter]` GUID Failures**:
   * 9 `[Error]` lines firing at engine startup (`Failed to create Subsumption variable 'List'`, `'Positions'`, `'PositionList'`, and missing task archetypes for specific GUIDs like `e0a668ab-90e3-4129-9bf9-0af0ae24ce36` and `b850f4d0-3fb7-46dc-a131-dbd1a4cad02c`).
2. **DGS RPC Registration Failures**:
   * `!!! Failed to register RPC [RPC.DGS...]` notices during initialization.

### Why It Looked Like the Culprit
Because the **Subsumption AI system** governs mission phase transitions, objective markers, and location proximity triggers, seeing Subsumption throw missing-GUID errors during boot looked like a textbook "ghost contract" bug: an uncleared contract GUID from a previous session corrupting the client's local Subsumption state before the player even joined a server.

---

## Phase 2: Cross-Account Comparison (`Bluefalcon2009` vs. Healthy Alt `bluefalcon2009_2`)

### Disproving the Initial Hypothesis
To test Phase 1, we ran `StarCitizenLogWatcher` against a known-good alternate account (`bluefalcon2009_2` in `example_logs/alt_Game.log`) and compared it against four broken logs from the main account (`Bluefalcon2009` in `Game.log`, `Game_NEW_NEW_OLD.log`, `Game_new_old.log`, and `Game_old.log`).

* **Result:** Both the broken main account and the healthy alt account produced the **exact same 9 `[Subsumption::ErrorReporter]` startup errors** and the **exact same `DGS RPC` notice**.
* **Why:** Those 9 Subsumption errors occur when the game engine boots and parses static mission XML templates from `Data.p4k`—long before account authentication or character selection. They are global client asset noise, **not** account-specific corruption.

### The Refined Hypothesis (Cross-Account Diff)
With startup Subsumption noise ruled out, we diffed post-login gameplay events between `bluefalcon2009_2` (healthy) and `Bluefalcon2009` (bugged). Four major differences emerged only on the broken main account:

1. **Missing `ObjectDataBank` & Orphan Marker Unbinds (`CObjectiveMarkerComponent`)**:
   * On the healthy alt account, accepting a mission (`Security Contractor Evaluation`) spawned the marker (`CLocalMissionPhaseMarker::CreateMarker`), streamed it in (`RWES`), and bound it via `<CObjectiveMarkerComponent::AddToPlayerDataBank>`.
   * On `Bluefalcon2009`, multiple accepted contracts streamed in (`RWES`) and **immediately fired `<CObjectiveMarkerComponent::RemoveFromPlayerDataBank>`** without `AddToPlayerDataBank` or `ObjectiveUpserted`.
   * Furthermore, `Game.log` (`05:45:30.558Z`) logged an explicit `[Error]` when detaching/rebinding the player entity:
     ```text
     [Error] <CObjectiveMarkerComponent::RemoveFromPlayerDataBank> MissionObjectiveMarker_65157[65157] - Player: Unavailable[0] has no ObjectDataBank [Team_MissionFeatures][Missions]
     ```
2. **False Zone-Exit / Proximity Loss Loops (`"New Objective: Return to <Location>"`)**:
   * **Co-op Masking (`Game_NEW_NEW_OLD.log`)**: During `Jorrit Dossier: Project Hyperion`, mission objectives progressed normally while a party member (`player_id 204772149061`) was present. At `05:23:37.533Z`, the party member left (`<PlayerLeft>`). **0.98 seconds later (`05:23:38.513Z`)**, while `Bluefalcon2009` was still inside Onyx Facility S2A7, the mission lost track of the player and spawned a marker ordering `Bluefalcon2009` to return to the facility entrance.
   * **Solo Looping (`Game.log`)**: During `Expel Outlaws` (`7d34cea8-...`), while `Bluefalcon2009` was standing inside the Storehouse looting crates, the game repeatedly thought the player had exited the zone and spawned `"New Objective: Return to Storehouse"` (`flags=ShowInLog|LocalOnlyObjective|`) **5 times in a row** (`06:38:44Z`, `06:39:39Z`, `06:42:25Z`, `06:45:06Z`, and `06:45:40Z`), before permanently stalling.
3. **Wikelo / Collector `<GenerateLocationProperty>` Storms**:
   * `alt_Game.log` logged `<GenerateLocationProperty>` only **12 times** at login.
   * `Bluefalcon2009` logged `<GenerateLocationProperty>` **568 to 3,758 times per session**, dumping batches of ~75 `contract: TheCollector_*` lines whenever contracts were accepted or abandoned.
4. **Login Metadata Error (`<GetSubscriptionKey Failed>`)**:
   * 3 out of 4 logs on `Bluefalcon2009` logged `[Error] <GetSubscriptionKey Failed> failed with error: UNAUTHENTICATED(16): 'No player Id was found in metadata'` right after character login, vs. 0 times on `alt_Game.log`.

---

## Phase 3: Same-Account Longitudinal Control (`Bluefalcon2009` Healthy vs. Bugged)

### Eliminating Account-Progression Differences
Comparing two *different* accounts in Phase 2 left one open variable: `Bluefalcon2009` is an established character with unlocked reputation chains, whereas `bluefalcon2009_2` is a fresh alt.

To eliminate account-progression differences, we ran `StarCitizenLogWatcher` against two historical logs from `logbackups` on the **exact same account (`Bluefalcon2009`)**:
* **Known Healthy Date (`2026-09-26`, `reports/should_be_good.md`)**: Captured before the account became bugged (missions completed normally).
* **Known Bugged Date (`2026-11-03`, `reports/should_be_bad.md`)**: Captured after the account became bugged.

### What the Same-Account Diff Proved (`reports/side_by_side_diff.txt`)

| Candidate Fingerprint | Sept 26 (`should_be_good.md`) | Nov 3 (`should_be_bad.md`) | Verdict |
| :--- | :--- | :--- | :--- |
| **`<GetSubscriptionKey Failed>`** | Present (`02:43:21Z`) | Absent | ❌ **Red Herring** (benign Social service metadata notice) |
| **`TheCollector_*` Location Storm** | Present (`252x` / `567x`) | Absent | ❌ **Red Herring** (Wikelo contract generator spam on accounts with Collector unlocks / server hiccups) |
| **Transient `RemoveFromPlayerDataBank`** | Fired at `02:55:22.169Z`, but **`AddToPlayerDataBank` followed 207ms later** (`02:55:22.376Z`); mission completed | Fired on markers without recovery | ⚠️ **Secondary Symptom** (benign if `AddToPlayerDataBank` immediately follows) |
| **`Missing Player ObjectDataBank` (`Player: Unavailable[0] has no ObjectDataBank`)** | **0 occurrences** | **3 `CRITICAL` occurrences** (`03:34:47Z`, `05:38:22Z`, `06:41:52Z`) | ✅ **PRIMARY SMOKING GUN** |
| **`Proximity Loss ('Return to' Loop)`** | **0 occurrences** | Present in `Game.log` (`5x` in Storehouse + `Exterior Zone 2`) | ✅ **PRIMARY SMOKING GUN** |

### The Exact Mechanism Captured in `should_be_bad.md`
In `reports/should_be_bad.md`, every single time `<CObjectiveMarkerComponent::RemoveFromPlayerDataBank> ... Player: Unavailable[0] has no ObjectDataBank` fires, the game is forced to re-bind the player and **re-broadcast `"Contract Accepted"` less than 1 second later** for missions the player had already accepted hours earlier:

1. **`03:34:47.198Z`**: `CRITICAL | Missing ObjectDataBank` (`MissionObjectiveMarker_910572666537`)
   $\rightarrow$ **`03:34:47.952Z` (+0.75s)**: Game re-fires `"Contract Accepted: Retake Platform"` (`7a53ae58-...`, originally accepted 51 minutes earlier at `02:43:19Z`).
2. **`05:38:22.830Z`**: `CRITICAL | Missing ObjectDataBank` (`MissionObjectiveMarker_910572726993`)
   $\rightarrow$ **`05:38:23.383Z` & `.662Z` (+0.55s)**: Game re-fires `"Contract Accepted"` for **both** `A Call to Arms` (`748d4341-...`) and `Retake Platform` (`7a53ae58-...`).
3. **`06:41:52.405Z`**: `CRITICAL | Missing ObjectDataBank` (`MissionObjectiveMarker_910572744450`)
   $\rightarrow$ **`06:41:52.863Z` & `53.242Z` (+0.46s)**: Game re-fires `"Contract Accepted"` for **both** `A Call to Arms` and `Retake Platform` a third time.

---

## Phase 4: Controlled A/B Reproduction — Project Hyperion (`hyperion_alt.log` vs. `hyperion_main.log`)

### Experimental Setup
To observe the exact moment the bug manifests in a controlled setting, we ran the **exact same gameplay loop** back-to-back on both accounts during **Project Hyperion** (`example_logs/hyperion_alt.log` on healthy alt `bluefalcon2009_2` vs. `example_logs/hyperion_main.log` on broken main `Bluefalcon2009`):
1. Accept **Project Hyperion** and Quantum Travel to the **ASD Onyx Facility**
2. Land in the hangar and enter the facility
3. Enter the **Engineering Wing**
4. Complete the first pipe-repair puzzle
5. Press the button on the terminal to complete the first puzzle objective
6. Exit to Main Menu $\rightarrow$ Exit Game

### Side-by-Side Timeline Comparison (`reports/hyperion_alt.md` vs. `reports/hyperion_main.md`)

| Gameplay Step | Healthy Alt (`bluefalcon2009_2` — `hyperion_alt.log`) | Broken Main (`Bluefalcon2009` — `hyperion_main.log`) |
| :--- | :--- | :--- |
| **1. Accept Project Hyperion** | **`04:47:16Z`**<br>• Mission `85f02505-...` accepted<br>• `MissionObjectiveMarker_924` bound via `AddToPlayerDataBank` (*"Go to Onyx Facility S2C7"*) | **`04:31:33Z`**<br>• Mission `de9ab375-...` accepted<br>• `MissionObjectiveMarker_969` bound via `AddToPlayerDataBank` (*"Go to Onyx Facility S1D10"*) |
| **2. Land at ASD Onyx Facility** | **`04:50:57Z`**<br>• `Marker 924` removed from `PlayerDataBank`<br>• Notification: `"Objective Complete: Go to Onyx Facility S2C7"` (`flags=ShowInLog\|LocalOnlyObjective\|`)<br>• `ObjectiveUpserted` (`04472ffe-...`): `"New Objective: Retrieve One of the Reagent Codes in Engineering Wing"` | **`04:35:08Z`**<br>• `Marker 969` removed from `PlayerDataBank`<br>• Notification: `"Objective Complete: Go to Onyx Facility S1D10"` (`flags=ShowInLog\|LocalOnlyObjective\|`)<br>• `ObjectiveUpserted` (`2d495953-...`): `"New Objective: Retrieve One of the Reagent Codes in Engineering Wing"` |
| **3. Enter Engineering Wing** *(First In-Game Symptom)* | **`04:52:49Z`**<br>• Parent objective updates to `flags=ShowInLog\|DisplayMarkerDistance\|AutoAddNewPlayers\|RespectInheritedVisibility\|`<br>• `ObjectiveUpserted` (`cf4f396f-...`, `created 1`): `"New Objective: Obtain Thermal Vent Diagnostic Data"`<br>• `MissionObjectiveMarker_922615341072` streams in (`RWES`) and **binds via `AddToPlayerDataBank`** | **`04:36:40Z – 04:38:00Z`**<br>• **TOTAL MISSION SILENCE.**<br>• Zero `ObjectiveUpserted`, zero `"Obtain Thermal Vent Diagnostic Data"` notification, and zero interior `MissionObjectiveMarker` stream-in (`RWES`) or `AddToPlayerDataBank`.<br>• At `04:37:55Z`, client logs **38 consecutive** `[Error] <Too many actions> Attempting to queue too many actions (11) on entity 'Bluefalcon2009'`. |
| **4. Repair Pipe & Press Terminal Button** | **`04:56:06Z`**<br>• `Marker 922615341072` streams out (`UFES`) & unbinds (`RemoveFromPlayerDataBank`)<br>• Notification: `"Downloading: <= UNINITIALIZED =>"`<br>• `ObjectiveUpserted` (`cf4f396f-...` $\rightarrow$ `COMPLETED`)<br>• `ObjectiveUpserted` (`5745ca0f-...` $\rightarrow$ `INPROGRESS`): `"New Objective: Obtain Thermal Condenser Diagnostic Data"`<br>• `MissionObjectiveMarker_922615359479` streams in (`RWES`) and **binds via `AddToPlayerDataBank`** | **`04:40:25Z – 04:41:20Z`**<br>• **TOTAL MISSION SILENCE.**<br>• Physically repairing the pipe and pressing the terminal button produces **no** `"Downloading"` HUD event, **no** `ObjectiveUpserted`, and **no** marker stream-in/out. |

### Why Step 2 Succeeds While Step 3–4 Fail (The Cascading In-Game Symptoms)
The controlled Hyperion run explains the exact chain of in-game symptoms—and why the terminal ultimately rejects the player with **"Not Authorized"**:

1. **Step 2 Succeeds — Client-Evaluated Outer Trigger (`LocalOnlyObjective`)**:
   * When arriving at the facility in Step 2, both logs show the arrival objective carries `flags=ShowInLog|LocalOnlyObjective|`:
     ```text
     [Notice] <CMissionLogEntry::UpdateActiveObjective> Objective updated id=..., flags=ShowInLog|LocalOnlyObjective|, hidden=0, hiddenInUI=0, markerHidden=0, uiDisplay[Priority=1][Text=Go to ~mission(Location)]
     ```
   * Because `LocalOnlyObjective` is evaluated client-side using the local player's position relative to the facility marker, the client knows you arrived and advances the mission to `"Retrieve One of the Reagent Codes in Engineering Wing"`.
2. **Symptom 1 — Entering Engineering Wing Fails to Trigger Sub-Objective (`AutoAddNewPlayers`)**:
   * On the healthy alt (`04:52:49Z`), crossing the threshold into the Engineering Wing triggers a **server-side Subsumption proximity volume**, updating the parent objective flags to `AutoAddNewPlayers`, pushing sub-objective `cf4f396f` (`"New Objective: Obtain Thermal Vent Diagnostic Data"`), and binding interior marker `MissionObjectiveMarker_922615341072` to `PlayerDataBank`.
   * On `Bluefalcon2009`, the server-side proximity volume never detects the player entering the Engineering Wing. Sub-objective `cf4f396f` is never created.
3. **Symptom 2 — Bridge Worms Never Spawn**:
   * On the healthy alt (`04:53:17Z` & `04:53:40Z`), jumping across the bridge triggers the encounter proximity volume (visible in the log as `CDeferredExplosionEffect::RequestRayCast` when the worms burst out).
   * On `Bluefalcon2009`, because the server does not detect the player inside the encounter volume (and the Engineering Wing phase never initialized), the worms never spawn.
4. **Symptom 3 — Terminal Displays `"Not Authorized"` on Button Press**:
   * Even though the pipe can still be physically repaired with the Multi-Tool, pressing **"Download Data"** on the terminal checks whether the player holds the active `"Obtain Thermal Vent Diagnostic Data"` sub-objective (`cf4f396f`).
   * Because Symptom 1 prevented that sub-objective from ever starting, the terminal rejects `Bluefalcon2009` with **"Not Authorized"** instead of starting `"Downloading: <= UNINITIALIZED =>"` (`04:56:06Z` on the alt).

### How This Same Mechanism Explains Other Mission Types (e.g., Nyx Ambush Missions)
This client-vs.-server trigger split explains why some missions appear to start normally and then stall completely as soon as you arrive:
* **Healthy Ambush Flow**: Accept mission $\rightarrow$ Quantum Travel to ambush beacon $\rightarrow$ exiting Quantum inside the ambush zone triggers the server-side Subsumption volume $\rightarrow$ mission updates with a countdown timer and a new marker to hide in the gas cloud $\rightarrow$ target spawns.
* **Bugged Ambush Flow**: Accept mission $\rightarrow$ Quantum Travel to ambush beacon $\rightarrow$ server-side Subsumption proximity volume never detects the player arriving at the beacon $\rightarrow$ no timer starts, no gas-cloud marker binds to `PlayerDataBank`, and no target spawns.

---

## Conclusion & Active Investigation Signatures

Across all four phases of analysis, the broken account state manifests in `Game.log` and `StarCitizenLogWatcher` reports through three distinct signatures:

1. **Structural Silence After `LocalOnlyObjective` Completion (Phase 4 — Controlled Hyperion Run)**:
   * **Healthy Account (`reports/hyperion_alt.md`)**: Initial `Marker DataBank Added` (`924`) $\rightarrow$ `Marker DataBank Removed` (`924`) on facility arrival $\rightarrow$ **subsequent `Marker DataBank Added` events (`922615341072`, `922615359479`)** as server-side interior sub-objectives trigger inside the facility.
   * **Bugged Account (`reports/hyperion_main.md`)**: Initial `Marker DataBank Added` (`969`) $\rightarrow$ `Marker DataBank Removed` (`969`) on facility arrival (`LocalOnlyObjective`) $\rightarrow$ **complete absence of any subsequent `Marker DataBank Added` or `ObjectiveUpserted` events** when entering interior mission zones or interacting with mission terminals.
2. **`Missing Player ObjectDataBank` During Active `Gameplay` (`active_investigation: true`)**:
   ```text
   [Error] <CObjectiveMarkerComponent::RemoveFromPlayerDataBank> MissionObjectiveMarker_... - Player: Unavailable[0] has no ObjectDataBank [Team_MissionFeatures][Missions]
   ```
   * When this fires during **`Gameplay`** (as seen 3 times in `reports/should_be_bad.md` and in `example_logs/Game.log`), the player entity's `ObjectDataBank` has been dropped/invalidated mid-session (`Player: Unavailable[0]`), severing active mission objective markers and forcing contract re-syncs less than 1 second later.
   * *(Note: If this line appears in state **`Main Menu`** ~1 second after `ExitToMenu` / `BeginAsyncReset`, it is normal level-unload teardown because the local player entity is destroyed before the remaining mission marker streams out.)*
3. **`Proximity Loss ('Return to' Loop)` (`active_investigation: true`)**:
   ```text
   [Notice] <SHUDEvent_OnNotification> Added notification "New Objective: Return to <Location>: " ...
   ```
   * When a mission uses a `LocalOnlyObjective` / zone-boundary tracker while the player's Subsumption/DataBank binding is broken (or right after a co-op partner leaves the party), the mission repeatedly thinks the player has exited the objective zone while they are standing directly inside it.



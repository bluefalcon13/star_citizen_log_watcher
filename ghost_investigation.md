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

## Phase 5: Tracing "Patient Zero" Across the Full Historical Archive (`-contracts` Mode & The Oct 11–14, 2026 Inflection Point)

### Building the `-contracts` Forensic Ledger & Hyperion Stall Detector
To pinpoint the exact day and event when `Bluefalcon2009` transitioned from a healthy account into the bugged state, we added two new forensic capabilities to `StarCitizenLogWatcher`:
1. **Hyperion Stall Detector (`Accept Hyperion -> No Deferred Explosion`)**:
   * Automatically flags any accepted **Project Hyperion** contract where the player never triggers `<CDeferredExplosionEffect::RequestRayCast>` (the Engineering Wing bridge worm ambush), proving the server-side interior proximity volume failed to fire before the mission ended or disconnected.
2. **Contract Ledger Mode (`-contracts`)**:
   * Scrapes every `<SHUDEvent_OnNotification> Added notification "Contract Accepted: ..."` and `<WaitForNotification>` across an entire directory of historical logs, cross-referencing each Mission ID against `<MissionEnded>`, `<EndMission>`, and Hyperion stall states to classify every contract's final outcome (`Completed`, `Failed`, `Abandoned`, `Stalled`, or `Unfinished`).

Running `StarCitizenLogWatcher.exe -d timeline_logs -contracts -o reports/timeline_contracts.md` ingested **4,213,685 log lines across 115 server sessions** (`Oct 1, 2026 – Nov 6, 2026`), generating a complete chronological ledger of **348 accepted contracts** (`reports/timeline_contracts.md`).

---

### Part A: The Oct 11, 2026 Mega-Group Grind — "Patient Zero" (`Game Build(12881860) 11 Oct 26 (06 38 58).log`)

In `timeline_logs/Game Build(12881860) 11 Oct 26 (06 38 58).log` (`06:38 AM – 11:37 AM MDT`, 1,513,909 lines—of which 1,462,252 lines from `L623–L1462875` were NPC eyebrow `<AggregateBindParentOutOfSync>` spam), `Bluefalcon2009` participated in a **35+ player mega-group grind** for RSI Discovery Month (`VisitingIasi`) points, accepting **110 shared contracts** in a single 5-hour session (`reports/oct11_grind.md`).

#### 1. The Two "Completed in Active Tab with Zero Objectives Left" Hung Contracts
Out of 110 accepted contracts, **7 shared party contracts** never sent `<EndMission>`, and **2 of them** specifically reached `MISSION_OBJECTIVE_STATE_COMPLETED` on their final phase/objective and removed their objective markers—yet the backend **never fired `<MissionEnded>` or `<EndMission>`**, leaving them permanently frozen in the mobiGlas **Active** tab with zero active objectives remaining:

* **Hung Completed Contract #1: `5c566b69-4f50-4bda-b366-ac87b50a943b` — `RSI Disc. Month: UCM Order (S)`** (`reports/oct11_grind.md`, Entry #53)
  * **Accepted:** `08:44:20 AM MDT` (`14:44:20Z`, `L1480586`), shared by `ownerId[204799005077]`.
  * **Final Objective Completed (`08:45:09 AM MDT` / `14:45:09.093Z`, `L1480761–L1480766`):**
    * `dropoff_246aa48e-a4d0-4669-be1f-8d4d029b34ef_0` transitioned to `MISSION_OBJECTIVE_STATE_COMPLETED`.
    * `MissionObjectiveMarker_33728` was removed from `Bluefalcon2009`'s `DataBank`.
    * `<ObjectiveComplete>` fired twice in the same millisecond for the final phase (`phase_246aa48e-a4d0-4669-be1f-8d4d029b34ef`, `state MISSION_OBJECTIVE_STATE_COMPLETED`).
  * **The Bug:** The backend **never sent `<MissionEnded>` or `<EndMission>`**. It remained stuck in the Active tab for the next 2 hours and 52 minutes with all objectives marked completed.

* **Hung Completed Contract #2: `85464525-a717-42a2-b168-3ec66570bfcb` — `RSI Disc. Month: UCM Order (M)`** (`reports/oct11_grind.md`, Entry #95)
  * **Accepted:** `10:31:37 AM MDT` (`16:31:37Z`, `L1503516`), shared by `ownerId[204755743322]`.
  * **Final Objective Completed (`10:34:42 AM MDT` / `16:34:42.434Z`, `L1504874–L1504894`):**
    * `phase_d0decb14-b110-4c94-90f3-f73ab2a117a5` completed, `MissionObjectiveMarker_70661` bound and unbound in the same tick (`16:34:42.434Z`), `"Objective Withdrawn: Locate Salvage Claim #86377"` fired (`L1504884`), and `objective_id 89493231-59d5-f2b9-d901-1911068cc0ae` transitioned to `MISSION_OBJECTIVE_STATE_COMPLETED` (`L1504894`).
  * **The Bug:** Again, **no `<MissionEnded>` or `<EndMission>`** ever fired. Worse, 10 minutes later (`10:44:24 AM MDT` / `16:44:24Z`, `L1506811–L1506819`), the original contract owner (`player_id 204755743322`) left the mission (`<PlayerLeft>` fired 3 times in a row for the owner), followed by dozens of other party members (`L1506506–L1512697`), completely orphaning the contract on the backend.

*(Five other shared contracts from the mega-party—`42302e80` [Entry #67], `f0b7a1c0` [Entry #80], `49772c40` [Entry #81], `50967525` [Entry #103], and `e77333eb` [Entry #110]—were also orphaned without `<EndMission>` as their party owners disconnected.)*

#### 2. The Final 12 Minutes of the Mega-Group Session (`11:24:47 AM – 11:37:10 AM MDT`)

| Time (MDT) | Time (UTC) | Line(s) | Exact Event in `Game Build(12881860) 11 Oct 26 (06 38 58).log` |
| :--- | :--- | :--- | :--- |
| **11:24:47 AM** | `17:24:47Z` | `L1511232` | Final working contract of the session (`054959e3`, `RSI Disc. Month: Relay Race`) completes normally. |
| **11:25:14 AM – 11:25:36 AM** | `17:25:14Z – 17:25:36Z` | `L1511322–L1511393` | `Bluefalcon2009` opens the mobiGlas Contracts Manager twice (`List 'MainTabList' requested variable '<unavailable>'`) and attempts to **Abandon** the hung contracts. The backend drops the abandon requests because the shared contracts are in a half-completed zombie state with departed owners. |
| **11:27:32 AM** | `17:27:32Z` | `L1511582–L1511583` | While quantum-traveling toward Crusader, two zombie markers (`872114345608` and `40120`) from 2-hour-old orphaned contract `42302e80` (accepted at `09:20 AM MDT`) spontaneously **re-inject themselves** (`AddToPlayerDataBank`) into `Bluefalcon2009`'s `ObjectDataBank`! |
| **11:30:55 AM – 11:36:20 AM** | `17:30:55Z – 17:36:20Z` | `L1512324–L1513202` | `Bluefalcon2009` arrives at **Orison**, enters the Armistice Zone (`11:32:14 AM`), receives `HangarXLTop` (`11:33:48 AM`), lands, and stores the ship. |
| **11:36:48 AM** | `17:36:48Z` | `L1513305–L1513340` | `Bluefalcon2009` leaves the mega-party (`"You have left the party."`), streaming out **34 `CPartyMarkerComponent` player markers** at once. **Even leaving the party fails to terminate any of the 7 hung contracts!** |
| **11:37:09 AM** | `17:37:09Z` | `L1513483–L1513542` | `Bluefalcon2009` executes **Exit to Menu** (`DisconnectCmd: disconnect light ExitToMenu`) to force-clear the Active Contracts tab:<br>• `17:37:09.515Z`: 3 zombie markers (`872114345608`, `40120`, `80543`) unbind just before entity teardown.<br>• `17:37:09.974Z`: `<CMissionLogEntry::UpdateActiveObjective>` fires mid-disconnect on **3 `LocalOnlyObjective` salvage claims** (`Locate Salvage Claim #~mission(ClaimNumber)`).<br>• `17:37:10.959Z`: **5 zombie markers** (`53457`, `70661` from hung completed contract `85464525`, `872155029873`, `52450`, and `76278`) crash in `RemoveFromPlayerDataBank` with **`Player: Unavailable[0] has no ObjectDataBank`**. |

---

### Part B: The Immediate Aftermath (`Oct 11 – Nov 6, 2026`)

Tracing `reports/timeline_contracts.md` immediately after that `ExitToMenu` reveals the exact five-stage progression of the bug taking hold on `Bluefalcon2009`:

1. **Stage 1 — First Post-Grind `Missing ObjectDataBank` & Re-Broadcast Loop (`Oct 11, 11:54:24 PM MDT` / `2026-10-12T05:54:24.717Z`)**:
   * Six seconds after quitting the mega-group session, `Bluefalcon2009` relaunched (`Game Build(12881860) 11 Oct 26 (11 37 30).log`) to play with friends.
   * Exiting to menu cleared the hung contracts from the mobiGlas UI, **but later in that very same log file** (`reports/timeline_contracts.md` Line 219 / **Entry #156**, `RSI Disc. Month: Orange Lvl. - Conduct Security Sweep`, `e7be357e-2bab-4486-80dc-557a4fa73f97`), `Bluefalcon2009` suffered an in-gameplay **`Missing ObjectDataBank` (`MissionObjectiveMarker_203102`)** and **`Re-broadcast Contract Accepted (2x)`** while `SC_Default` was actively running, stalling the mission.
2. **Stage 2 — First Noticeable Broken Solo Contracts (`Oct 12, 11:00:41 PM MDT` & `Oct 13, 08:57:21 AM MDT`)**:
   * In `Game Build(12881860) 12 Oct 26 (22 25 37).log` (`reports/timeline_contracts.md` Line 221 / **Entry #158**, `2026-10-13T05:00:41Z`), `Bluefalcon2009` accepted **`RSI Disc. Month: Take Out Gang Leader`** (`d262b614-e5f4-48a2-9001-8df1b9f72460`). It immediately hit **`Proximity Loss ('Return to' Loop)`** (`05:08:32Z`) and had to be **Abandoned** (`05:37:49Z`)—the exact moment the player first noticed *"all these missions are broken."*
   * The next morning (`Oct 13, 08:57:21 AM MDT` / `2026-10-13T14:57:21Z`, `reports/timeline_contracts.md` Line 235 / **Entry #172**), a second attempt at **`RSI Disc. Month: Take Out Gang Leader`** (`0f3f9df1-c6c8-491e-a484-932f1ebd2c86`) failed with the exact same **`Proximity Loss ('Return to' Loop)`** and was **Abandoned**.
3. **Stage 3 — Diagnosing the "Ghost" State in Project Hyperion & Calling in a "Non-Ghost" Friend (`Oct 14, 12:09:37 AM MDT` / `2026-10-14T06:09:37Z`)**:
   * Right after those `Take Out Gang Leader` failures, in `Game Build(12913470) 13 Oct 26 (19 41 35).log` (`reports/timeline_contracts.md` Line 242 / **Entry #179**), `Bluefalcon2009` accepted **`Jorrit Dossier: Project Hyperion [BP]*`** (`dd3b95b6-c73b-4089-9305-4df8f2e850cd`) and entered the facility solo.
   * Seeing the exact same missing interior triggers a friend had experienced the previous patch, `Bluefalcon2009` immediately recognized the bug as the **"Ghost" state** and called a friend into the mission to act as the **"non-ghost" proximity trigger**, allowing `dd3b95b6` to complete at `07:25:00Z` (`01:25:00 AM MDT`).
4. **Stage 4 — Repeated Solo Project Hyperion Stalls (`Oct 16 – Oct 31, 2026`)**:
   * Every subsequent solo test of **`Jorrit Dossier: Project Hyperion`** without a "non-ghost" co-op partner stalled inside the Engineering Wing and had to be abandoned:
     * **Entry #204** (`2026-10-16T06:29:59Z`, Line 267) — **Abandoned** (`9153c12e-...`)
     * **Entry #205** (`2026-10-17T02:00:24Z`, Line 268) — **Abandoned** (`eb9dcb4b-...`, **`HYPERION STALL (No Worm Spawn)`**)
     * **Entries #219 & #220** (`2026-10-19T05:35:24Z`, Lines 282–283) — **Stalled / Abandoned** (`c3a7f920-...`, **`Proximity Loss ('Return to' Loop); HYPERION STALL (No Worm Spawn)`**)
     * **Entry #241** (`2026-10-20T23:43:09Z`, Line 304) — **Stalled** (`d74e1154-...`, **`HYPERION STALL (No Worm Spawn)`**)
     * **Entry #248** (`2026-10-22T05:05:48Z`, Line 311) — **Abandoned** (`2a2f0d91-...`, **`HYPERION STALL (No Worm Spawn)`**)
     * **Entry #314** (`2026-10-31T19:00:29Z`, Line 377) — **Stalled** (`7e7a0dd6-...`, **`HYPERION STALL (No Worm Spawn)`**)
   * Conversely, when a co-op partner was present (**Entry #317**, `2026-11-01T02:53:31Z`, Line 380), `Project Hyperion` (`037f8e12-...`) **Completed** even while `Bluefalcon2009` logged `Missing ObjectDataBank; Re-broadcast Contract Accepted (2x)` mid-run.
5. **Stage 5 — Chronic Account-Wide `Missing ObjectDataBank` & `Proximity Loss` Loops (`Oct 16 – Nov 6, 2026`)**:
   * Across the rest of the ledger (`Entries #203, #213, #216, #277, #280, #301, #303, #312, #316, #324, #325, #328, #329, #333, #336, #347, #348`), multi-stage surface and bunker contracts (`Reputation Management`, `Jorrit Dossier: Updated Seismic Data`, `Verified Bounty: Mahmud Amo at QV Breaker Station`, `Retake Platforms`, `High-Risk Bounty: Dan Akter`, `Expel Outlaws`) repeatedly suffered **`Missing ObjectDataBank`**, **`Re-broadcast Contract Accepted`**, and **`Proximity Loss ('Return to' Loop)`**.

---

## Conclusion & Active Investigation Signatures

Across all five phases of analysis, the broken account state on `Bluefalcon2009` was triggered by **orphaned shared party contracts with completed objectives and zombie `ObjectDataBank` markers during the Oct 11, 2026 35-player grind**, and manifests in `Game.log` and `StarCitizenLogWatcher` reports through four deterministic signatures:

1. **Hyperion Stall (`Accept Hyperion -> No Deferred Explosion`) & Structural Silence After `LocalOnlyObjective` Completion**:
   * **Healthy Account (`reports/hyperion_alt.md`)**: Initial `Marker DataBank Added` (`924`) $\rightarrow$ `Marker DataBank Removed` (`924`) on facility arrival $\rightarrow$ **subsequent `Marker DataBank Added` events (`922615341072`, `922615359479`)** and `<CDeferredExplosionEffect::RequestRayCast>` (bridge worms) as server-side interior sub-objectives trigger inside the facility.
   * **Bugged Account (`reports/hyperion_main.md`)**: Initial `Marker DataBank Added` (`969`) $\rightarrow$ `Marker DataBank Removed` (`969`) on facility arrival (`LocalOnlyObjective`) $\rightarrow$ **complete absence of any subsequent `Marker DataBank Added`, `ObjectiveUpserted`, or `RequestRayCast` events** when entering interior mission zones or interacting with mission terminals (`"Not Authorized"`).
2. **`Missing Player ObjectDataBank` During Active `Gameplay` (`active_investigation: true`)**:
   ```text
   [Error] <CObjectiveMarkerComponent::RemoveFromPlayerDataBank> MissionObjectiveMarker_... - Player: Unavailable[0] has no ObjectDataBank [Team_MissionFeatures][Missions]
   ```
   * First appeared mid-gameplay on `2026-10-12T05:54:24.717Z` (`Game Build(12881860) 11 Oct 26 (11 37 30).log#L98129`), hours after the Oct 11 mega-group zombie-marker disconnect crash.
   * When this fires during **`Gameplay`** (as also seen 3 times in `reports/should_be_bad.md` and in `example_logs/Game.log`), the player entity's `ObjectDataBank` has been dropped/invalidated mid-session (`Player: Unavailable[0]`), severing active mission objective markers and forcing contract re-syncs less than 1 second later.
   * *(Note: If this line appears in state **`Main Menu`** ~1 second after `ExitToMenu` / `BeginAsyncReset`, it is level-unload teardown because the local player entity is destroyed before remaining mission markers stream out.)*
3. **`Proximity Loss ('Return to' Loop)` (`active_investigation: true`)**:
   ```text
   [Notice] <SHUDEvent_OnNotification> Added notification "New Objective: Return to <Location>: " ...
   ```
   * When a mission uses a `LocalOnlyObjective` / zone-boundary tracker while the player's Subsumption/DataBank binding is broken (or 0.98s after a co-op partner leaves the party), the mission repeatedly thinks the player has exited the objective zone while they are standing directly inside it.
4. **Orphaned Shared Contract Desync (`<ObjectiveComplete>` Without `<EndMission>` + Departed Party Owner)**:
   * The root-cause catalyst captured in `Game Build(12881860) 11 Oct 26 (06 38 58).log`: when shared party missions complete their final objective (`MISSION_OBJECTIVE_STATE_COMPLETED`) in a massive party without firing `<MissionEnded>` / `<EndMission>`, and the original `ownerId` subsequently leaves the mission (`<PlayerLeft>`), the contract becomes un-abandonable in the client's Active tab and leaves zombie `MissionObjectiveMarker` bindings attached to the player's `ObjectDataBank`.




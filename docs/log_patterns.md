# Log Analysis Patterns

## Overview
This document defines the regex patterns used to identify specific system states, errors, and critical failures within the game logs.

## 1. Critical System Failures

### 1.1 DGS Service Failures
Identifies failures in the Distributed Game Service (DGS) RPC registrations, which often precede a disconnect or crash.
* **Regex:** `!!! Failed to register RPC \[RPC\.DGS\..*\]`
* **Example:** `<2026-10-20T08:08:32.264Z> !!! Failed to register RPC [RPC.DGS.CommArray.Manage]`

### 1.2 Subsumption UUID Errors
Identifies errors in the AI Subsumption system where a specific GUID (UUID) fails to map to an archetype, variable, or dependency.
* **Regex:** `\[Error\] <\[Subsumption::ErrorReporter\]> .* \(GUID:[a-f0-9\-]{36}\) .* \[Team_CoreAI\]\[Subsumption\]`
* **Example:** `<2026-10-18T13:54:04.860Z> [Error] <[Subsumption::ErrorReporter]> List File: '...' - Failed to create Subsumption variable 'List' (GUID:40a7a5ca-59da-11f1-55d5-d3a0356e3f93) ... [Team_CoreAI][Subsumption]`

### 1.3 Subsumption Dependency/Archetype Mismatch
Specifically catches cases where a UUID exists but the associated asset/archetype is missing from the loaded library.
* **Regex:** `Failed to create Subsumption (task|variable|dependency) .* associated with the guid '([a-f0-9\-]{36})'`
* **Example:** `<2026-10-18T13:54:04.908Z> [Error] ... Failed to create Subsumption task because the Archetype associated with the guid 'e0a668ab-90e3-4129-9bf9-0af0ae24ce36' is not present`

---

## 2. Contract & Mission Lifecycle

### 2.1 Contract Completion/Failure
Matches logs where a contract is explicitly closed by the client (either successfully or via failure).
* **Regex:** `\[CContractGenerator::CContractGenerator\] .* (completed|failed|closed)`
* **Example:** `<2026-10-18T13:54:01.462Z> [Notice] <CContractGenerator::CContractGenerator> ... [Missions] (Status: completed)`

### 2.2 Contract Result Validation
Used to correlate a contract state change with its specific outcome result.
* **Regex:** `(Contract|Mission) .* (result|outcome) is (Success|Failure|Completed|Failed)`
* **Note:** This pattern is used to verify if the game engine's reported result matches the contract's final state.

---

## Pattern Legend
| Severity | Description |
| :--- | :--- |
| **CRITICAL** | System-level failure (DGS, Crash-inducing) |
| **ERROR** | Logic failure (Subsumption UUID/Archetype mismatches) |
| **INFO/NOTICE** | Expected lifecycle transitions (Contract status changes) |
# 📜 Go Coding Standards: Star Citizen Log Watcher
**Strict Enforcement Required. No Deviations.**

## 1. Architecture & Design
* **Idiomatic Go:** Write robust, readable, and idiomatic Go code.
* **Dependency Injection:** Prefer small, decoupled packages. Use constructor functions (e.g., `NewProcessor(...)`) to inject dependencies (channels, config, logger) rather than allowing components to reach into global state.
* **Interface Segregation:** Use small, consumer-defined interfaces to keep components decoupled.
* **No Global State:** Avoid package-level `init()` functions and global variables. All state must be contained within structs and passed explicitly.

## 2. Control Flow
* **The Happy Path:** Use guard clauses and early returns to handle errors and edge cases.
* **Minimize Nesting:** Keep indentation levels shallow. If a function is deeply nested, extract logic into a named helper function.

## 3. Concurrency & Lifecycle
* **Managed Goroutines:** Never "fire-and-forget" a goroutine. Every goroutine must have a managed lifecycle.
* **Context-Awareness:** Pass `context.Context` through all long-running processes to handle timeouts, cancellations, and shutdowns gracefully.
* **Synchronization:** Use `sync.WaitGroup` or `golang.org/x/sync/errgroup` to coordinate the lifecycle of multiple goroutines.
* **Channel Ownership:** Clearly define which component owns a channel. The "producer" is responsible for closing the channel to signal completion. 
* **Explicit Buffering:** Keep channels unbuffered unless a specific performance bottleneck has been identified and a buffered channel is explicitly justified in the design.

## 4. Error Handling
* **Zero Tolerance:** Always check `err`. Never ignore it.
* **Type Assertions:** Always use the "comma-ok" idiom for type assertions: `val, ok := x.(T)`.
* **Error Wrapping:** Use `fmt.Errorf("context: %w", err)` to provide stack-like context while preserving the original error for inspection.
* **No Double-Handling:** Do not log an error and then return it; this leads to log bloat. Either handle/log it, or return it.
* **Panic Policy:** **Do not panic.** Treat `panic` as a last resort for truly unrecoverable system states (e.g., failed fundamental dependency injection).

## 5. Types & Structs
* **Explicit Initialization:** Always use explicit field names in struct initializers (e.g., `User{Name: "Bob"}` instead of `User{"Bob"}`).
* **Nil Slices:** Return `nil` for empty slices to follow standard Go idioms.
* **Documentation:** Every exported type, constant, and function must have a descriptive comment following Go doc conventions.

## 6. Implementation Integrity
* **NO PLACEHOLDERS:** Generate complete, fully compilable Go code. 
* **No Omissions:** Never use `// TODO`, `// ... existing code ...`, or placeholder comments for function bodies. If a function is requested, provide the logic required to satisfy the design.
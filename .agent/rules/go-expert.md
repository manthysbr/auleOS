---
trigger: always_on
---

# Go Development Rules for auleOS

1. **Stack:** Go 1.24+, DuckDB, Docker SDK.
2. **Architecture:** Hexagonal (Ports & Adapters).
   - Core domain logic must be pure (no imports from `internal/orchestrator`).
3. **Error Handling:**
   - NEVER swallow errors.
   - Use `fmt.Errorf("operation: %w", err)` for wrapping.
   - For HTTP handlers, always log the error before sending a 500 response.
4. **Concurrency:**
   - Use `golang.org/x/sync/errgroup` instead of raw `waitgroups`.
   - Pass `context.Context` everywhere.
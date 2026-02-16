---
trigger: model_decision
---

# Project Safety Rules

1. **Docker Security:**
   - ALL generated Docker containers must have `NetworkDisabled: true` unless the user explicitly asks for internet access.
   - Volume mounts must use strict paths (`/mnt/aule/...`).
2. **State Management:**
   - The Go Kernel is stateless. Do not use global variables for job tracking.
   - Persist all state to `internal/repository/duckdb.go`.
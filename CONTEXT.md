auleOS Business & Architectural Vision
1. Product Vision: The "Glass Box" Agentic OS
auleOS (named after Aulë, the Smith and Maker) is a local-first Agentic Operating System designed for high-performance content creation and orchestration.

Unlike "black box" AI tools (like ChatGPT or Midjourney), auleOS is a "Glass Box": it exposes the internal reasoning, resource consumption, and tool execution of every AI agent in real-time. It treats AI Agents not as chat bots, but as observable system processes.

Core Value Proposition
Infrastructure-Level Observability: A "Zabbix for Agents." Users can see GPU usage, token latency, and step-by-step tool execution logs for every job.

Deterministic Execution: Agents run in isolated, ephemeral environments (Docker Containers). If a task succeeds once, it must succeed again.

Privacy & Sovereignty: Runs 100% locally on WSL + Docker, utilizing local LLMs (Ollama) by default, with optional cloud bursting.

Multi-Modal Crafting: Native support for text, code, documents (PDF/MD), video (FFmpeg), and audio generation pipelines.

2. The "Triad" Architecture
The system is composed of three decoupled planes:

A. The Kernel (Control Plane)
Role: The brain. Stateless, high-concurrency orchestrator.

Tech: Go 1.24+.

Responsibilities:

Job Scheduling (Weighted Semaphores).

Worker Lifecycle Management (Spawn/Kill/Reap).

Telemetry Aggregation (NATS/Channels).

State Persistence (DuckDB).

Constraint: The Kernel NEVER processes heavy media directly. It delegates to Workers.

B. The Workers (Execution Plane)
Role: The muscle. Ephemeral, specialized, and isolated environments.

Tech: Docker + "Watchdog" Sidecar (Go HTTP Server).

Responsibilities:

Execute specific tasks (e.g., "Summarize PDF", "Render Video").

Report strict progress updates via the Watchdog.

Read/Write data solely via Shared Volumes.

Constraint: Workers act as "Function-as-a-Service" units. They have NO network access unless explicitly whitelisted in their Manifest.

C. The Interface (Experience Plane)
Role: The monitor.

Tech: Next.js (Web) + Bubbletea (Terminal) via Server-Sent Events (SSE).

Responsibilities:

Visualize the "Agent State Tree" (Thinking -> Tooling -> Result).

Provide manual intervention ("Human-in-the-Loop") for checkpoints.

Display real-time resource metrics (VRAM, CPU, Cost).

3. Domain Language & Concepts
AWU (Agentic Work Unit): The atomic unit of work. A specific task assigned to a specific worker (e.g., "Transcode video.mp4 to 720p").

The Manifest: A strict JSON contract defined by each Worker Image, detailing its capabilities (video_gen, doc_parse), resource needs (vram_mb), and required inputs.

The Watchdog: A lightweight Go HTTP server running inside every worker container. It bridges the gap between the Kernel (JSON commands) and the tool (Shell/Python scripts).

The Ledger: The immutable record of all jobs, logs, and artifacts, stored in DuckDB. This allows the system to survive crashes and restarts without losing job history.

Zombie Reaping: The Kernel's startup protocol that identifies and kills orphaned Docker containers that do not match the current Ledger state.

4. Technical Constraints & Standards
Data Transport (The "Volume First" Rule)
Strict Rule: JSON Payloads via HTTP must NEVER exceed 1MB.

Mechanism: All heavy assets (PDFs, Videos, Images) are passed via Docker Shared Volumes mounted at /mnt/aule/workspace/{job_id}.

Flow: Kernel writes file to disk -> Sends path in JSON to Worker -> Worker reads from disk -> Worker writes result to disk -> Worker returns path in JSON.

Security (Zero-Trust)
Network: All Workers spawn with --network none by default.

Filesystem: Workers are Read-Only (except for the workspace volume and /tmp).

User: Workers run as non-root user aule.

Observability Standards
Every Go function in the Kernel must emit structured logs (slog).

Every Worker execution must stream progress updates (0-100%) to the Kernel via the Watchdog.

5. Development Philosophy: Spec-Driven Development (SDD)
We do not write code without a spec.

Define: Create the Interface (Go) or Schema (JSON/YAML) first.

Verify: Ensure the spec meets the architectural constraints.

Implement: Generate the code to satisfy the spec.

Test: Validate against the spec using TDD.
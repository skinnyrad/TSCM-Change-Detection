# Copilot Instructions for TSCM-Change-Detection

Purpose: provide repository-specific instructions so future Copilot sessions can act confidently.

---

Build, test, and lint commands

- Backend (Go):
  - Requires Go 1.25+ (see go.mod).
  - Build production binary (embeds frontend/dist):
    - go build -o tscm-change-detection .
  - Run locally (single command):
    - go run .
  - Run a single Go test (if tests are added):
    - go test -run ^TestName$ ./path/to/package
  - Run full test suite (if any):
    - go test ./...
  - Quick vet/format:
    - go vet ./...
    - go fmt ./...

- Frontend (React + Bun):
  - Install deps and build (required before go build to embed fresh UI):
    - cd frontend && bun install && bun run build
  - Dev (hot-reload):
    - cd frontend && bun run dev
    - Backend in dev: go run . (backend listens on :8080, frontend dev server uses :3000)

- Notes about embedding: the Go binary uses `//go:embed all:frontend/dist`. Always run the frontend build before `go build` if making UI changes.

- Python (optional): requirements.txt exists (Pillow, streamlit, OpenCV, numpy). These appear to be for auxiliary scripts/notebooks — not required for the Go+React app.

---

High-level architecture (big picture)

- Root Go service (main.go)
  - Uses Gin (HTTP router), CORS configured to allow http://localhost:3000 for frontend dev.
  - Exposes API under /api with handlers implemented in internal/api.
  - Embeds frontend/dist into the binary and serves the SPA as static files; SPA fallback serves index.html for unknown routes.

- internal/
  - api: HTTP handler layer (endpoints wired in main.go). Key handlers (as named in main.go):
    - HandleUploadBefore, HandleUploadAfter — receive uploaded images
    - HandleAnalyze — perform analysis operations
    - HandleWarp, HandleClearWarp — alignment/warp control
    - HandleImageBefore, HandleImageAfter — serve the stored images
  - imgproc: image processing algorithms (pure-Go; no OpenCV runtime dependency for the Go server)
  - state: storage/state management for uploads/warp points (in-memory/file-backed as implemented)

- frontend/
  - React + TypeScript (React 19). Uses MUI for UI components and Bun as the dev/build tool.
  - Build output placed in frontend/dist and embedded by the Go server.
  - Dev: Bun hot-reload runs on :3000; backend proxies /api/* (CORS allowance already present in main.go).

- Data flow summary:
  1. User uploads Before and After images in the SPA.
  2. Frontend POSTs to /api/upload/before and /api/upload/after.
  3. Backend stores images and exposes them at /api/image/before and /api/image/after.
  4. Frontend requests /api/analyze to run image-difference algorithms in imgproc; results returned as JSON for visualization.
  5. If needed, frontend can POST warp control points to /api/warp and /api/clear-warp.

---

Key conventions and repository-specific gotchas

- Frontend embedding: building the frontend is a required pre-step for creating an up-to-date production binary. The server embeds frontend/dist at compile time.

- CORS/dev proxy: main.go explicitly allows http://localhost:3000. When running the frontend dev server, keep it on port 3000 or update the AllowOrigins config.

- Handler naming: API handlers follow the `HandleXxx` naming in internal/api and are mounted under /api in main.go. Use those names when searching for behavior.

- No-opencv runtime for server: image processing in the Go server is implemented in Go (bild, golang.org/x/image, etc.). The presence of opencv-python in requirements.txt is for the Python tooling, not the Go server.

- Bun usage: frontend build and dev commands use Bun. The package.json defines `dev` and `build` scripts; prefer `bun run build` for production output.

- Ports and addresses:
  - Backend default: :8080 (see main.go)
  - Frontend dev server: :3000 (CORS allowed)

- Common quick searches for Copilot prompts:
  - Where API endpoints are wired: main.go lines that call api.Handle*
  - Image processing implementations: internal/imgproc
  - State management: internal/state
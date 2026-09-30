# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

TSCM Change Detection: a web app that compares a Before/After photo and highlights changes. Go (Gin) backend with pure-Go image processing (no OpenCV), React 19 + MUI frontend built with Bun. The final product is a single self-contained Go binary that embeds the built frontend.

## Commands

The Go binary embeds `frontend/dist` via `//go:embed all:frontend/dist` (`main.go`), so **`frontend/dist` must exist (run the frontend build) before `go build`/`go run`/`go vet` will compile**. `dist/` is gitignored.

```bash
# Production build
cd frontend && bun install && bun run build && cd ..
go build -o tscm-change-detection .
./tscm-change-detection          # serves UI + API on http://localhost:8080

# Dev with hot reload: two processes
go run .                          # backend on :8080
cd frontend && bun run dev        # Bun dev server on :3000, proxies /api/* to :8080

go vet ./...
go test -race ./...
cd frontend && bun run typecheck && bun test   # TS typecheck + unit tests
go test -run '^TestName$' ./internal/imgproc
```

`.github/workflows/ci.yml` runs typecheck, tests, vet and build on PRs. Releases are built by `.github/workflows/release.yml` on `v*.*.*` tags (builds frontend once, cross-compiles 6 OS/arch targets). `scripts/prototyping/requirements.txt` (Python/OpenCV/streamlit) is for auxiliary prototyping only, not the app. `test-images/` has sample before/after pairs for manual testing.

## Architecture

**Backend** (`main.go` → `internal/`):
- `main.go` wires all routes under `/api` and serves the embedded SPA, falling back to `index.html` for unknown paths. CORS only allows `http://localhost:3000` (the dev server); change it if the dev port changes.
- `internal/api/handler.go`: `HandleXxx` Gin handlers. Endpoints: `upload/{before,after,baseline}`, `baselines/clear`, `registration` (auto-align settings), `analyze`, `analyze/alternate` (diff, subtraction, heatmap, Canny, contours in one pass), `warp`, `auto-warp`, `clear-warp`, `image/{before,after}`. `parseDiffOpts` clamps every analysis parameter and attaches the alignment's validity mask. Uploads are size-capped, rejected above 120 MP, and EXIF-rotated (`imgproc/orientation.go`).
- `internal/state/store.go`: a **process-global in-memory singleton** (`state.Global`). Raw images are immutable once stored. Derived data (the aligned pair, manual warp, combined baselines) is computed from a `Snapshot` outside the lock and only accepted by `SetAligned`/`SetWarpedBefore` if the store's `generation` hasn't moved, so concurrent uploads can't leave a mismatched pair. There is no per-user separation: the app is single-user by design.
- `internal/imgproc/`: pure-Go algorithms. The pipeline is `register.go` (`RegisterPair`: aspect-preserving resize → feature homography with plausibility gates → residual translation search, optional `localflow.go` block refinement; returns a validity mask so warp borders never count as change) then `diff.go` (`ComputeDiffV2`: exposure match → blur → CIELAB diff with optional shift tolerance → baseline-spread subtraction → exclusion masks → fixed or MAD-based adaptive threshold → open/close → min-region). `regions.go` is the single connected-component labeler (8-connectivity) used by stats, filtering, contours and region ranking. `autohomography.go` supplies the feature matcher and RANSAC.

**Batch mode** (many photos of one scene → find the odd ones out): `imgproc/batch.go` (`AnalyzeBatchFunc`) picks the medoid image as anchor, registers every image to it, exposure-matches, then iterates a robust "golden set" (per-pixel median + MAD over images that agree; images whose score is a robust outlier are dropped and stats recomputed). Each image's score is the peak of its per-pixel z-map after two detectors: a grey-level opening for blobs (removes thin registration residue) and a thin-structure pass that keeps long components lying on edges the golden reference lacks (wires/cables). Clipped highlights are excluded. An image is anomalous if its score clears `max(median + ImageZ·spread, PixelZ·MinScoreFactor)` and it has at least one region. `state/batch.go` stores only a 1600 px JPEG per image (analysis decodes on demand, runs in a background goroutine with progress); `api/batch.go` serves results and aligned/heat/reference images. The manual-alignment dialog's tie points come from `imgproc/tiepoints.go` (per-region corner detection → one stable point per border cell, After positions from the full fit).

**Frontend** (`frontend/src`): React + MUI. `App.tsx` owns shared `DetectionSettings` (`lib/settings.ts`, which also builds the request form) and hosts the tabs in `components/`. `hooks/` wrap API calls; `lib/api.ts` is the one fetch helper. `hooks/useFullscreen` + `hooks/useFlipKeys` implement wrapper-level fullscreen and ←/→ flipping (used by `ImageComparisonTab` and `ResultImage`). `ImageOverlay` draws ranked region boxes and ignore zones in image fractions. `lib/report.ts` builds the exported HTML report.

**Flow**: upload → server decodes, rotates, stores → `realign()` registers the pair → frontend posts settings to `/api/analyze*` → renders the result plus ranked regions.

## Evaluating detection changes

`internal/imgproc/eval_test.go` scores the pipeline on the change-detection lab. It is skipped unless `LAB_DIR` points at the lab root (the folder containing `solutions/summary.json`):

```bash
LAB_DIR=~/projects/skinnyrd/classes/ai-for-tscm-2/Files-AFT2/change-detection \
  CFG="reg=1,match=1,color=10,athr=1,thr=0" go test ./internal/imgproc -run TestEvalLab -v
```

Other env-gated harnesses in `internal/imgproc` (all skipped by default; keep data in `tmp/`):
- `TestEvalAlignment` (`ALIGN_IMAGES=<dir>`): auto-alignment error against synthetic ground-truth homographies, plus tie-point spread.
- `TestEvalBatch` (`BATCH_DIR=<dir of images>`, optional `BATCH_TRUTH=<x-ray results.json>`): batch anomaly detection; the x-ray set (`xrays.zip` from the anomaly-detection course folder) should give 8/8 anomalies, 0 false positives.
- `TestEvalBatchSynthetic` (`BATCH_SCENE=<photo>`, `BATCH_DONOR=<photo>`, optional `BATCH_WIRE=1`, `BATCH_OBJ_DIV=<n>`): 60 jittered shots of one scene with objects or thin wires planted in 6.
- `TestDumpTiePoints` / `TestDumpFeatures` (`TIE_A`, `TIE_B`, `TIE_OUT`/`TIE_FEAT`): draw tie points / detected features for visual inspection.

`CFG` keys: `reg` (auto-register), `local`, `match` (exposure match), `color` (Lab weight ×10), `shift`, `athr` (adaptive threshold), `thr`, `border` (‰). The reference boxes come from the course's OpenCV solution, not hand labels, so use the numbers to compare runs, not as absolute accuracy. Keep scratch work in `tmp/` (git-ignored).

## Conventions

`.gitignore` is written for this repo: every rule is anchored with a leading `/` to the folder it applies to. Don't paste in generic templates. The old Python template's unanchored `lib/` silently excluded `frontend/src/lib/` from a commit. Run `scripts/check-ignored.sh` after adding folders; it fails if any source file is being ignored.

Agent guidance files here are named `AGENTS.md` (root and `frontend/`), not `CLAUDE.md`. The frontend uses Bun (see `frontend/AGENTS.md`); don't add Vite or Express.

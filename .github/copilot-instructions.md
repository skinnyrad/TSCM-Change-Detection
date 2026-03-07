# Copilot instructions for TSCM-Change-Detection

Purpose
- Streamlit-based image change detection tool for TSCM analysts. The single entrypoint is app.py which runs the web UI and contains all core analysis code.

Build / Run / Test / Lint
- Setup environment: python3 -m venv venv && source venv/bin/activate
- Install runtime deps: pip install -r requirements.txt
- Run the app (development):
  - streamlit run app.py
- Tests: No test suite included in this repository. If/when pytest tests are added, run a single test with:
  - pytest tests/test_filename.py::test_name
  - or run a subset via pytest -k <expr>
- Linting: No linter or pre-commit configuration present in the repo; add tools (flake8/ruff/black) if you want enforced formatting or checks.

High-level architecture (big picture)
- Single-process Streamlit application (app.py) that:
  1. Accepts two uploaded images (Before and After) via Streamlit file_uploaders.
  2. Loads images with PIL and converts to RGB numpy arrays (load_image).
  3. Aligns image sizes by resizing the Before image to match the After image when needed (align_images).
  4. Computes change representations:
     - Grayscale absolute difference + binary threshold (compute_difference)
     - Float image subtraction normalized to 0–255 (apply_image_subtraction)
     - Heatmap conversion of grayscale diffs (diff_to_heatmap)
  5. Generates overlays and visual summaries (highlight_changes, draw_contours_on) and computes simple stats (change_stats).
  6. Presents UI in three Streamlit tabs: Image Comparison, Change Detection, Advanced Analysis.
- Key third-party runtime dependencies: Pillow, streamlit, opencv-python (cv2), numpy, streamlit-image-comparison.

Key conventions and repository-specific patterns
- Single-file app: app.py contains UI, utilities, and analysis functions; changes to behavior typically require editing this file.
- Image color space: PIL -> numpy produces RGB arrays; OpenCV color constants are used assuming RGB arrays (cv2.COLOR_RGB2GRAY / cv2.COLOR_BGR2RGB conversions appear when applying color maps).
- Alignment rule: The code always resizes the Before image to the After image when shapes differ (align_images). Be aware resizing uses INTER_LANCZOS4 which preserves detail but may alter pixel-level diff sensitivity.
- Thresholding and sensitivity:
  - Default sensitivity value is 30 for threshold-based methods.
  - Threshold sliders are exposed in the UI (5–100) and applied as integer thresholds to grayscale diffs.
- Contours and region counting rely on cv2.findContours with RETR_EXTERNAL; very small noisy blobs may be removed by morphological open with a 5×5 kernel (compute_difference).
- UI session state keys used by the app:
  - show_after (toggle state for comparison tab)
  - adv_sens, canny_low, canny_high (advanced analysis control state)
- Accepted upload formats: jpg, jpeg, png (enforced by file_uploader).

Files to glance at when making changes
- app.py — main application and all logic.
- requirements.txt — runtime dependency list.
- README.md — user-facing usage examples and screenshots.

When editing or extending
- Keep heavy image-processing changes isolated in small functions (compute_difference, apply_image_subtraction, diff_to_heatmap, highlight_changes, draw_contours_on). The UI layer (main) wires these functions into Streamlit controls and should remain thin.
- If adding tests, factor computational logic into pure functions that accept/return numpy arrays so unit tests can run without Streamlit.

Other AI/assistant config files
- None of the common assistant instruction files (CLAUDE.md, .cursorrules, AGENTS.md, .windsurfrules, CONVENTIONS.md, AIDER_CONVENTIONS.md, .clinerules) are present in the repo root as of this commit.

Questions
- Would you like an MCP server configured (e.g., Playwright for browser-driven end-to-end testing of the Streamlit UI)?

If you want edits or more coverage (adding example pytest tests, CI workflow, or recommended linting/pre-commit config), say which area to expand and adjustments will be made.
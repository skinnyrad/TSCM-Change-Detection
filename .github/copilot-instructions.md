# Copilot instructions for TSCM-Change-Detection

## Purpose

Streamlit-based image change-detection tool for TSCM (Technical Surveillance Countermeasures) analysts. Compares Before/After photos to identify modifications or anomalies in a surveillance area.

## Build / Run / Test / Lint

- Setup environment: `python3 -m venv venv && source venv/bin/activate`
- Install deps: `pip install -r requirements.txt`
- Run the app: `streamlit run app.py`
- Tests: No test suite yet. If pytest tests are added, run with:
  - `pytest tests/test_filename.py::test_name` (single test)
  - `pytest -k <expr>` (subset)
- Linting: No linter or pre-commit configuration is present.

## Repository layout

```
app.py              # Single-file Streamlit app — all UI and analysis logic
requirements.txt    # Runtime deps: Pillow, streamlit, opencv-python, numpy, streamlit-image-comparison==0.0.4
README.md           # User-facing docs with screenshots
img/                # Screenshot assets referenced by README
test-images/        # Sample before/after image pairs for manual testing
.github/            # GitHub config (this file)
```

## Architecture

Single-process Streamlit application (`app.py`):

1. **Image upload** — two `file_uploader` widgets accept jpg/jpeg/png Before and After images.
2. **Loading** — `load_image()` opens via PIL, converts to RGB, returns a numpy array.
3. **Alignment** — `align_images()` resizes the Before image to the After image's dimensions using `INTER_LANCZOS4`.
4. **Analysis functions** (pure numpy/OpenCV, no Streamlit dependency):
   - `compute_difference(img1, img2, threshold)` → grayscale absolute diff + binary threshold (with 5×5 morphological open to reduce noise).
   - `apply_image_subtraction(img1, img2)` → float subtraction normalized to 0–255.
   - `diff_to_heatmap(diff_gray)` → JET colormap (BGR→RGB converted).
   - `highlight_changes(img2, thresh)` → red overlay blended onto After image.
   - `draw_contours_on(img, thresh)` → green contours via `findContours(RETR_EXTERNAL)`.
   - `change_stats(thresh)` → dict with `pct`, `changed_px`, `regions`.
5. **UI** — `main()` wires the above into three Streamlit tabs:
   - **Image Comparison** — slider (via `streamlit-image-comparison`) or toggle mode.
   - **Change Detection** — four methods (Basic Difference, Image Subtraction, Threshold Detection, Heat Map) with live metrics.
   - **Advanced Analysis** — Canny edge detection on diff + contour overlay with adjustable thresholds.

## Key conventions

- **Single-file app**: all code lives in `app.py`. Changes to behavior almost always mean editing this file.
- **Color space**: PIL→numpy produces RGB arrays. OpenCV colormap outputs (BGR) are explicitly converted to RGB before display.
- **Alignment rule**: Before image is always resized to match After image dimensions — never the reverse.
- **Default sensitivity**: 30 for all threshold-based methods; UI slider range is 5–100.
- **Session state keys**: `show_after`, `adv_sens`, `canny_low`, `canny_high`.

## When editing or extending

- Keep image-processing logic in small pure functions that accept/return numpy arrays. The `main()` function should remain a thin UI wiring layer.
- If adding tests, target the pure analysis functions (`compute_difference`, `apply_image_subtraction`, etc.) so tests run without Streamlit.
- Update `README.md` if adding new analysis tabs or changing the UI structure — it documents each tab with screenshots.
# TSCM Change Detection Analysis Tool

A Streamlit web application for Technical Surveillance Countermeasures (TSCM) professionals to detect and analyze changes between two images. Upload a Before and After photo to identify potential modifications or anomalies in a surveillance area.

![Upload Screen](./img/main1.png)

![Analysis Results](./img/main2.png)



## Installation

**Prerequisites:** Python 3.8+

1. Clone or download the repository.

2. Create a virtual environment:

   ```bash
   python3 -m venv venv
   ```

3. Activate the virtual environment:

   - On macOS/Linux:
     ```bash
     source venv/bin/activate
     ```
   - On Windows:
     ```bash
     venv\Scripts\activate
     ```

4. Install dependencies:

   ```bash
   pip install -r requirements.txt
   ```

5. Run the application:

   ```bash
   streamlit run app.py
   ```

The app opens at `http://localhost:8501`.



## Usage

Upload a **Before** and **After** image using the two file pickers at the top. Previews with pixel dimensions appear immediately. If the images differ in size, the Before image is automatically resized to match the After image before any analysis.



## Analysis Tools

### Tab 1 — Image Comparison

Visually compare the two images side by side. Switch between two modes using the **Comparison Mode** radio:

- **Slider** — drag a divider left/right to reveal Before or After
- **Toggle** — click `Before`, `After`, or `↔ Toggle` buttons to flip between full-resolution images

![Image Comparison](./img/image-comparison.png)



### Tab 2 — Change Detection

All methods display three live metrics at the top — **Changed Area %**, **Changed Pixels**, and **Distinct Regions** — before showing the visual results.

#### Basic Difference
Shows a grayscale difference map, a binary thresholded mask, and a red highlight overlay on the After image.

![Basic Difference](./img/basic-difference.png)

#### Heat Map
Renders change intensity as a JET colormap (blue = low change, red = high change) alongside the red highlight overlay. Useful for gauging the magnitude of changes, not just their location.

![Heat Map](./img/heat-map.png)

#### Image Subtraction
Performs a pixel-wise float subtraction (After − Before), normalized to 0–255. Preserves gradient information rather than producing a binary result — useful for detecting subtle, gradual modifications.

![Image Subtraction](./img/image-subtraction.png)

#### Threshold Detection
Applies a user-defined sensitivity threshold to the raw difference, showing only changes above that level alongside the highlight overlay.

![Threshold Detection](./img/threshold-detection.png)

**Detection Sensitivity slider (5–100, default 30):** applies to Basic Difference, Heat Map, and Threshold Detection.
- Lower values → more sensitive, flags subtle changes
- Higher values → fewer false positives, only significant changes



### Tab 3 — Advanced Analysis

Provides two simultaneous views for structural analysis of detected changes.

- **Edge Enhancement** — runs Canny edge detection on the difference image. Canny Low (20–150) and High (50–300) thresholds are adjustable.
- **Change Contours** — draws green contours around changed regions on the After image, with a region count in the caption.

![Advanced Analysis](./img/advanced-analysis.png)

A **Detection Threshold** slider at the top of this tab controls which pixels are considered changed for both views.



## Best Practices

- Use consistent lighting, camera position, and angle between shots
- Start with **Basic Difference** for an initial read, then refine sensitivity
- Use **Heat Map** to assess the severity and spread of changes
- Use **Advanced Analysis** contours to identify and count distinct changed objects
- If getting too many false positives, increase the sensitivity threshold; if missing subtle changes, decrease it

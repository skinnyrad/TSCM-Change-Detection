import streamlit as st
import cv2
import numpy as np
from PIL import Image
from streamlit_image_comparison import image_comparison

st.set_page_config(page_title="TSCM Change Detection", layout="wide")


# ── Image Loading ──────────────────────────────────────────────────────────────

def load_image(image_file):
    img = Image.open(image_file)
    if img.mode != "RGB":
        img = img.convert("RGB")
    return np.array(img)


# ── Core Utilities ─────────────────────────────────────────────────────────────

def align_images(img1, img2):
    """Resize img1 to match img2 dimensions if they differ."""
    if img1.shape[:2] != img2.shape[:2]:
        img1 = cv2.resize(
            img1, (img2.shape[1], img2.shape[0]), interpolation=cv2.INTER_LANCZOS4
        )
    return img1, img2


def to_gray(img):
    return cv2.cvtColor(img, cv2.COLOR_RGB2GRAY)


# ── Analysis Functions ─────────────────────────────────────────────────────────

def compute_difference(img1, img2, threshold=30):
    img1, img2 = align_images(img1, img2)
    diff = cv2.absdiff(to_gray(img1), to_gray(img2))
    _, thresh = cv2.threshold(diff, threshold, 255, cv2.THRESH_BINARY)
    kernel = np.ones((5, 5), np.uint8)
    thresh = cv2.morphologyEx(thresh, cv2.MORPH_OPEN, kernel)
    return diff, thresh


def apply_image_subtraction(img1, img2):
    img1, img2 = align_images(img1, img2)
    if img1.ndim != img2.ndim:
        if img1.ndim == 2:
            img1 = cv2.cvtColor(img1, cv2.COLOR_GRAY2RGB)
        if img2.ndim == 2:
            img2 = cv2.cvtColor(img2, cv2.COLOR_GRAY2RGB)
    diff = img2.astype(np.float32) - img1.astype(np.float32)
    return cv2.normalize(diff, None, 0, 255, cv2.NORM_MINMAX).astype(np.uint8)


def diff_to_heatmap(diff_gray):
    """Convert a grayscale diff image to a JET colormap (RGB)."""
    heatmap_bgr = cv2.applyColorMap(diff_gray, cv2.COLORMAP_JET)
    return cv2.cvtColor(heatmap_bgr, cv2.COLOR_BGR2RGB)


def highlight_changes(img2, thresh, color=(255, 60, 60), alpha=0.55):
    """Blend a red overlay onto img2 wherever thresh > 0."""
    overlay = img2.copy()
    overlay[thresh > 0] = color
    return cv2.addWeighted(img2, 1.0 - alpha, overlay, alpha, 0)


def draw_contours_on(img, thresh):
    contours, _ = cv2.findContours(
        thresh, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE
    )
    result = img.copy()
    cv2.drawContours(result, contours, -1, (0, 255, 0), 2)
    return result, contours


def change_stats(thresh):
    total = thresh.size
    changed = int(np.sum(thresh > 0))
    contours, _ = cv2.findContours(
        thresh, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE
    )
    return {
        "pct": round(changed / total * 100, 2),
        "changed_px": changed,
        "regions": len(contours),
    }


# ── Main App ───────────────────────────────────────────────────────────────────

def main():
    st.title("TSCM Change Detection Analysis")
    st.caption("Upload a Before and After image to identify changes between them.")

    # ── Upload Section ─────────────────────────────────────────────────────────
    col1, col2 = st.columns(2)
    with col1:
        f1 = st.file_uploader("Before Image", type=["jpg", "jpeg", "png"])
    with col2:
        f2 = st.file_uploader("After Image", type=["jpg", "jpeg", "png"])

    if not (f1 and f2):
        st.info("Upload both a **Before** and **After** image to begin analysis.")
        return

    img1 = load_image(f1)
    img2 = load_image(f2)

    # Image previews with dimensions
    pc1, pc2 = st.columns(2)
    pc1.image(
        img1,
        caption=f"Before — {img1.shape[1]}×{img1.shape[0]} px",
        use_container_width=True,
    )
    pc2.image(
        img2,
        caption=f"After — {img2.shape[1]}×{img2.shape[0]} px",
        use_container_width=True,
    )

    if img1.shape[:2] != img2.shape[:2]:
        st.warning(
            f"Images have different sizes "
            f"({img1.shape[1]}×{img1.shape[0]} vs {img2.shape[1]}×{img2.shape[0]}). "
            "The Before image will be automatically resized to match the After image for analysis."
        )

    st.divider()
    st.header("Analysis Tools")
    tab1, tab2, tab3 = st.tabs(["Image Comparison", "Change Detection", "Advanced Analysis"])

    # ── Tab 1: Visual Comparison ───────────────────────────────────────────────
    with tab1:
        st.subheader("Visual Comparison")
        mode = st.radio(
            "Comparison Mode",
            ["Slider", "Toggle"],
            horizontal=True,
            help="**Slider**: drag to reveal before/after. **Toggle**: click buttons to flip between images.",
        )

        if mode == "Slider":
            image_comparison(
                img1=Image.fromarray(img1),
                img2=Image.fromarray(img2),
                label1="Before",
                label2="After",
            )
        else:
            # Toggle mode — track which image is shown via session state
            if "show_after" not in st.session_state:
                st.session_state.show_after = False

            b1, b2, b3, _ = st.columns([1, 1, 1, 4])
            with b1:
                if st.button(
                    "Before",
                    use_container_width=True,
                    type="secondary" if st.session_state.show_after else "primary",
                ):
                    st.session_state.show_after = False
            with b2:
                if st.button(
                    "After",
                    use_container_width=True,
                    type="primary" if st.session_state.show_after else "secondary",
                ):
                    st.session_state.show_after = True
            with b3:
                if st.button("↔ Toggle", use_container_width=True):
                    st.session_state.show_after = not st.session_state.show_after

            current_img = img2 if st.session_state.show_after else img1
            current_label = "After" if st.session_state.show_after else "Before"
            st.image(current_img, caption=current_label, use_container_width=True)

    # ── Tab 2: Change Detection ────────────────────────────────────────────────
    with tab2:
        st.subheader("Change Detection Results")

        method = st.selectbox(
            "Detection Method",
            ["Basic Difference", "Image Subtraction", "Threshold Detection", "Heat Map"],
        )

        # Sensitivity slider for diff-based methods
        sensitivity = 30
        if method != "Image Subtraction":
            sensitivity = st.slider(
                "Detection Sensitivity (threshold)",
                5, 100, 30,
                help="Lower = more sensitive to small changes. Higher = fewer false positives.",
            )

        diff, thresh = compute_difference(img1, img2, threshold=sensitivity)
        stats = change_stats(thresh)

        # Change statistics summary
        m1, m2, m3 = st.columns(3)
        m1.metric("Changed Area", f"{stats['pct']}%")
        m2.metric("Changed Pixels", f"{stats['changed_px']:,}")
        m3.metric("Distinct Regions", stats["regions"])
        st.divider()

        if method == "Basic Difference":
            c1, c2, c3 = st.columns(3)
            c1.image(diff, caption="Difference Map", use_container_width=True)
            c2.image(thresh, caption="Thresholded Changes", use_container_width=True)
            c3.image(
                highlight_changes(img2, thresh),
                caption="Changes Highlighted on After",
                use_container_width=True,
            )

        elif method == "Image Subtraction":
            subtracted = apply_image_subtraction(img1, img2)
            c1, c2 = st.columns(2)
            c1.image(subtracted, caption="Subtraction Result", use_container_width=True)
            c2.image(
                highlight_changes(img2, thresh),
                caption="Changes Highlighted on After",
                use_container_width=True,
            )

        elif method == "Threshold Detection":
            c1, c2 = st.columns(2)
            c1.image(
                thresh,
                caption=f"Threshold Result (sensitivity={sensitivity})",
                use_container_width=True,
            )
            c2.image(
                highlight_changes(img2, thresh),
                caption="Changes Highlighted on After",
                use_container_width=True,
            )

        elif method == "Heat Map":
            heatmap = diff_to_heatmap(diff)
            c1, c2 = st.columns(2)
            c1.image(heatmap, caption="Change Intensity Heat Map", use_container_width=True)
            c2.image(
                highlight_changes(img2, thresh),
                caption="Changes Highlighted on After",
                use_container_width=True,
            )

    # ── Tab 3: Advanced Analysis ───────────────────────────────────────────────
    with tab3:
        st.subheader("Advanced Analysis")

        adv_sens = st.slider(
            "Detection Threshold", 5, 100, 30, key="adv_sens",
            help="Controls which pixels are considered changed for edge and contour analysis.",
        )
        diff_adv, thresh_adv = compute_difference(img1, img2, threshold=adv_sens)
        img1_aligned, _ = align_images(img1, img2)

        # Sliders row
        s1, s2 = st.columns(2)
        with s1:
            canny_low = st.slider("Canny Low Threshold", 20, 150, 100, key="canny_low")
        with s2:
            canny_high = st.slider("Canny High Threshold", 50, 300, 200, key="canny_high")

        # Images row
        c1, c2 = st.columns(2)

        with c1:
            st.markdown("**Edge Enhancement**")
            if canny_low >= canny_high:
                st.warning("Low threshold must be less than High threshold.")
            else:
                edges = cv2.Canny(diff_adv, canny_low, canny_high)
                st.image(edges, caption="Edge Detection on Diff", use_container_width=True)

        with c2:
            st.markdown("**Change Contours**")
            result_after, contours = draw_contours_on(img2, thresh_adv)
            st.image(
                result_after,
                caption=f"Contours on After Image ({len(contours)} regions detected)",
                use_container_width=True,
            )


if __name__ == "__main__":
    main()

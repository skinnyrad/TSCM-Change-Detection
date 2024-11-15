import streamlit as st
import cv2
import numpy as np
from PIL import Image
import io
from streamlit_image_comparison import image_comparison

def load_image(image_file):
    img = Image.open(image_file)
    return np.array(img)

def compute_difference(img1, img2):
    # Ensure images are same size
    img1 = cv2.resize(img1, (img2.shape[1], img2.shape[0]))
    
    # Convert to grayscale
    gray1 = cv2.cvtColor(img1, cv2.COLOR_RGB2GRAY)
    gray2 = cv2.cvtColor(img2, cv2.COLOR_RGB2GRAY)
    
    # Calculate absolute difference
    diff = cv2.absdiff(gray1, gray2)
    
    # Apply threshold to highlight significant changes
    _, thresh = cv2.threshold(diff, 30, 255, cv2.THRESH_BINARY)
    
    # Apply some noise reduction
    kernel = np.ones((5,5), np.uint8)
    thresh = cv2.morphologyEx(thresh, cv2.MORPH_OPEN, kernel)
    
    return diff, thresh

def apply_image_subtraction(img1, img2):
    # Ensure images are the same size
    img1 = cv2.resize(img1, (img2.shape[1], img2.shape[0]))

    # Convert images to the same number of channels (if necessary)
    if len(img1.shape) != len(img2.shape):
        if len(img1.shape) == 2:  # img1 is grayscale
            img1 = cv2.cvtColor(img1, cv2.COLOR_GRAY2RGB)
        if len(img2.shape) == 2:  # img2 is grayscale
            img2 = cv2.cvtColor(img2, cv2.COLOR_GRAY2RGB)
    
    # Convert to float32 for subtraction
    f_img1 = img1.astype(np.float32)
    f_img2 = img2.astype(np.float32)
    
    # Perform subtraction
    diff = cv2.subtract(f_img2, f_img1)
    
    # Normalize to 0-255 range
    diff_norm = cv2.normalize(diff, None, 0, 255, cv2.NORM_MINMAX)
    return diff_norm.astype(np.uint8)

def main():
    st.title("TSCM Change Detection Analysis")
    
    # File uploaders
    col1, col2 = st.columns(2)
    with col1:
        image1 = st.file_uploader("Upload First Image", type=['jpg', 'jpeg', 'png'])
    with col2:
        image2 = st.file_uploader("Upload Second Image", type=['jpg', 'jpeg', 'png'])
        
    if image1 and image2:
        img1 = load_image(image1)
        img2 = load_image(image2)
        
        st.header("Analysis Tools")
        
        tab1, tab2, tab3 = st.tabs(["Image Comparison", "Change Detection", "Advanced Analysis"])
        
        with tab1:
            st.subheader("Visual Comparison")
            image_comparison(
                img1=Image.fromarray(img1),
                img2=Image.fromarray(img2),
                label1="First Image",
                label2="Second Image"
            )
            
        with tab2:
            st.subheader("Change Detection Results")
            
            method = st.selectbox(
                "Select Detection Method",
                ["Basic Difference", "Image Subtraction", "Threshold Detection"]
            )
            
            if method == "Basic Difference":
                diff, thresh = compute_difference(img1, img2)
                st.image(diff, caption="Difference Map", use_container_width=True)
                st.image(thresh, caption="Thresholded Changes", use_container_width=True)
                
            elif method == "Image Subtraction":
                subtracted = apply_image_subtraction(img1, img2)
                st.image(subtracted, caption="Subtraction Result", use_container_width=True)
                
            elif method == "Threshold Detection":
                sensitivity = st.slider("Detection Sensitivity", 10, 100, 30)
                diff, thresh = compute_difference(img1, img2)
                _, thresh = cv2.threshold(diff, sensitivity, 255, cv2.THRESH_BINARY)
                st.image(thresh, caption="Threshold Detection Result", use_container_width=True)
        
        with tab3:
            st.subheader("Advanced Analysis")
            
            col1, col2 = st.columns(2)
            with col1:
                enhance = st.checkbox("Enable Edge Enhancement")
                if enhance:
                    diff, _ = compute_difference(img1, img2)
                    edges = cv2.Canny(diff, 100, 200)
                    st.image(edges, caption="Edge Detection", use_container_width=True)
            
            with col2:
                contour = st.checkbox("Show Change Contours")
                if contour:
                    diff, thresh = compute_difference(img1, img2)
                    contours, _ = cv2.findContours(thresh, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
                    result = img2.copy()
                    cv2.drawContours(result, contours, -1, (0, 255, 0), 2)
                    st.image(result, caption="Change Contours", use_container_width=True)

if __name__ == "__main__":
    main()

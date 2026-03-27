package api

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/skinnyrad/tscm-change-detection/internal/imgproc"
)

type dims struct {
	W int `json:"w"`
	H int `json:"h"`
}

type analyzeResponse struct {
	Stats      imgproc.Stats     `json:"stats"`
	Images     map[string]string `json:"images"`
	BeforeDims dims              `json:"before_dims"`
	AfterDims  dims              `json:"after_dims"`
	Resized    bool              `json:"resized"`
}

func HandleAnalyze(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse form: " + err.Error()})
		return
	}

	// Decode before image
	beforeFile, _, err := c.Request.FormFile("before")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing 'before' image"})
		return
	}
	defer beforeFile.Close()

	beforeImg, _, err := image.Decode(beforeFile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to decode 'before' image: " + err.Error()})
		return
	}

	// Decode after image
	afterFile, _, err := c.Request.FormFile("after")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing 'after' image"})
		return
	}
	defer afterFile.Close()

	afterImg, _, err := image.Decode(afterFile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to decode 'after' image: " + err.Error()})
		return
	}

	// Parse parameters
	method := c.PostForm("method")
	if method == "" {
		method = "basic"
	}
	validMethods := map[string]bool{
		"basic": true, "subtraction": true, "threshold": true,
		"heatmap": true, "advanced": true,
	}
	if !validMethods[method] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid method: %q", method)})
		return
	}

	sensitivity := parseIntDefault(c.PostForm("sensitivity"), 30)
	if sensitivity < 5 || sensitivity > 100 {
		sensitivity = 30
	}

	cannyLow := parseIntDefault(c.PostForm("canny_low"), 100)
	cannyHigh := parseIntDefault(c.PostForm("canny_high"), 200)

	if method == "advanced" && cannyLow >= cannyHigh {
		c.JSON(http.StatusBadRequest, gin.H{"error": "canny_low must be less than canny_high"})
		return
	}

	// Record original dimensions
	ab := beforeImg.Bounds()
	bb := afterImg.Bounds()
	beforeDims := dims{W: ab.Dx(), H: ab.Dy()}
	afterDims := dims{W: bb.Dx(), H: bb.Dy()}

	// Align images
	before, after, resized := imgproc.Align(beforeImg, afterImg)

	// Compute diff and threshold mask (used by all methods for stats)
	diff, thresh := imgproc.ComputeDiff(before, after, uint8(sensitivity))
	stats := imgproc.ChangeStats(thresh)

	images := map[string]string{}
	highlightColor := [3]uint8{255, 60, 60}

	switch method {
	case "basic":
		if s, err := imgproc.EncodeBase64PNG(diff); err == nil {
			images["diff_map"] = s
		}
		if s, err := imgproc.EncodeBase64PNG(thresh); err == nil {
			images["threshold_mask"] = s
		}
		highlighted := imgproc.HighlightChanges(after, thresh, highlightColor, 0.55)
		if s, err := imgproc.EncodeBase64PNG(highlighted); err == nil {
			images["highlight"] = s
		}

	case "subtraction":
		subtracted := imgproc.Subtract(before, after)
		if s, err := imgproc.EncodeBase64PNG(subtracted); err == nil {
			images["subtraction"] = s
		}
		highlighted := imgproc.HighlightChanges(after, thresh, highlightColor, 0.55)
		if s, err := imgproc.EncodeBase64PNG(highlighted); err == nil {
			images["highlight"] = s
		}

	case "threshold":
		if s, err := imgproc.EncodeBase64PNG(thresh); err == nil {
			images["threshold_mask"] = s
		}
		highlighted := imgproc.HighlightChanges(after, thresh, highlightColor, 0.55)
		if s, err := imgproc.EncodeBase64PNG(highlighted); err == nil {
			images["highlight"] = s
		}

	case "heatmap":
		heatmap := imgproc.JETColormap(diff)
		if s, err := imgproc.EncodeBase64PNG(heatmap); err == nil {
			images["heatmap"] = s
		}
		highlighted := imgproc.HighlightChanges(after, thresh, highlightColor, 0.55)
		if s, err := imgproc.EncodeBase64PNG(highlighted); err == nil {
			images["highlight"] = s
		}

	case "advanced":
		edges := imgproc.CannyEdge(diff, uint8(cannyLow), uint8(cannyHigh))
		if s, err := imgproc.EncodeBase64PNG(edges); err == nil {
			images["edges"] = s
		}
		contoured, _ := imgproc.DrawContours(after, thresh, [3]uint8{0, 255, 0})
		if s, err := imgproc.EncodeBase64PNG(contoured); err == nil {
			images["contours"] = s
		}
	}

	c.JSON(http.StatusOK, analyzeResponse{
		Stats:      stats,
		Images:     images,
		BeforeDims: beforeDims,
		AfterDims:  afterDims,
		Resized:    resized,
	})
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/skinnyrad/tscm-change-detection/internal/api"
)

//go:embed all:frontend/dist
var frontendDist embed.FS

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	// CORS: allow the Bun dev server (localhost:3000) during development
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:3000"},
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{"Content-Type"},
	}))

	// API routes
	apiGroup := r.Group("/api")
	apiGroup.POST("/upload/before", api.HandleUploadBefore)
	apiGroup.POST("/upload/after", api.HandleUploadAfter)
	apiGroup.POST("/analyze", api.HandleAnalyze)
	apiGroup.POST("/analyze/diff", api.HandleAnalyzeDiff)
	apiGroup.POST("/analyze/subtraction", api.HandleAnalyzeSubtraction)
	apiGroup.POST("/analyze/heatmap", api.HandleAnalyzeHeatmap)
	apiGroup.POST("/analyze/canny", api.HandleAnalyzeCanny)
	apiGroup.POST("/warp", api.HandleWarp)
	apiGroup.POST("/clear-warp", api.HandleClearWarp)
	apiGroup.GET("/image/before", api.HandleImageBefore)
	apiGroup.GET("/image/after", api.HandleImageAfter)

	// Serve embedded React frontend for all other routes (SPA fallback)
	subFS, err := fs.Sub(frontendDist, "frontend/dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(subFS))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Check if the file exists in the embedded FS
		f, err := subFS.Open(path[1:]) // strip leading /
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// SPA fallback: serve index.html for all unmatched routes
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	const addr = ":8080"
	fmt.Println("TSCM Change Detection is available at http://localhost:8080")
	r.Run(addr)
}

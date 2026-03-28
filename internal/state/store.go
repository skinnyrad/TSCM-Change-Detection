package state

import (
	"image"
	"sync"
	"sync/atomic"
)

// Dims holds pixel dimensions of an image.
type Dims struct {
	W int
	H int
}

// Store holds server-side image state. Images are decoded and aligned once on
// upload so that analyze calls can skip expensive decode/resize work.
type Store struct {
	mu sync.RWMutex

	// rawBefore/rawAfter are the original decoded images, kept for warp input
	// and for serving display PNGs.
	rawBefore *image.NRGBA
	rawAfter  *image.NRGBA

	// alignedBefore/alignedAfter are pre-processed for ComputeDiff (computed
	// once whenever both images are available).
	alignedBefore *image.NRGBA
	alignedAfter  *image.NRGBA
	resized       bool

	// warpedBefore replaces alignedBefore in analyze after a perspective warp.
	warpedBefore *image.NRGBA

	beforeDims Dims
	afterDims  Dims

	// Independent version counters for cache-busting the display PNG endpoints.
	beforeVersion atomic.Int64
	afterVersion  atomic.Int64
}

// Global is the single shared store for the process.
var Global = &Store{}

// SetBefore stores the decoded before image and clears stale alignment data.
// The caller should call SetAligned immediately if the after image is also present.
func (s *Store) SetBefore(img *image.NRGBA, dims Dims) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rawBefore = img
	s.beforeDims = dims
	s.warpedBefore = nil
	s.alignedBefore = nil
	s.alignedAfter = nil
	s.beforeVersion.Add(1)
}

// SetAfter stores the decoded after image and clears stale alignment data.
// The caller should call SetAligned immediately if the before image is also present.
func (s *Store) SetAfter(img *image.NRGBA, dims Dims) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rawAfter = img
	s.afterDims = dims
	s.warpedBefore = nil
	s.alignedBefore = nil
	s.alignedAfter = nil
	s.afterVersion.Add(1)
}

// SetAligned stores the pre-aligned image pair (computed once per upload pair).
func (s *Store) SetAligned(before, after *image.NRGBA, resized bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alignedBefore = before
	s.alignedAfter = after
	s.resized = resized
}

// SetWarpedBefore stores a perspective-warped before image (same dims as after).
func (s *Store) SetWarpedBefore(warped *image.NRGBA) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.warpedBefore = warped
}

// ClearWarp removes the warped image so analysis reverts to alignedBefore.
func (s *Store) ClearWarp() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.warpedBefore = nil
}

// AnalysisPair returns the (before, after) pair ready for ComputeDiff.
// Returns nil, nil, false if alignment has not been computed yet.
func (s *Store) AnalysisPair() (before, after *image.NRGBA, resized bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.alignedBefore == nil || s.alignedAfter == nil {
		return nil, nil, false
	}
	b := s.alignedBefore
	if s.warpedBefore != nil {
		b = s.warpedBefore
	}
	return b, s.alignedAfter, s.resized
}

// RawBefore returns the raw before image (for warp input and display serving).
func (s *Store) RawBefore() *image.NRGBA {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rawBefore
}

// RawAfter returns the raw after image (for warp output sizing and display serving).
func (s *Store) RawAfter() *image.NRGBA {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rawAfter
}

// Dims returns the original image dimensions.
func (s *Store) Dims() (bDims, aDims Dims, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.rawBefore == nil || s.rawAfter == nil {
		return Dims{}, Dims{}, false
	}
	return s.beforeDims, s.afterDims, true
}

// HasImages returns true once both images are uploaded and alignment is computed.
func (s *Store) HasImages() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.alignedBefore != nil && s.alignedAfter != nil
}

func (s *Store) BeforeVersion() int64 { return s.beforeVersion.Load() }
func (s *Store) AfterVersion() int64  { return s.afterVersion.Load() }

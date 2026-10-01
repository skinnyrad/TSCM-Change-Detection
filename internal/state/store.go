package state

import (
	"image"
	"sync"
	"sync/atomic"

	"github.com/skinnyrad/tscm-change-detection/internal/imgproc"
)

// Dims holds pixel dimensions of an image.
type Dims struct {
	W int
	H int
}

// Settings are the registration choices that affect the aligned pair.
type Settings struct {
	AutoRegister bool // feature-based alignment on upload (default on)
	LocalRefine  bool // non-rigid block refinement after auto alignment
}

// Store holds server-side image state. Images are decoded and aligned once so
// analyze calls can skip expensive decode/resize/registration work.
//
// Images are treated as immutable once stored: callers receive the same
// pointers and must not modify pixels. Every mutation goes through a method
// that takes the lock, and derived data (the aligned pair) is only accepted
// if the raw inputs it was computed from are still current, so concurrent
// uploads can never leave a mismatched pair behind.
type Store struct {
	mu sync.RWMutex

	rawBefore, rawAfter   *image.NRGBA
	beforeDims, afterDims Dims

	// baselines are extra "before" sweeps (beyond rawBefore) for multi-baseline mode.
	baselines []*image.NRGBA

	aligned *imgproc.Aligned // nil until both images are present and registered
	warped  *imgproc.Aligned // manual-warp override of aligned.Before (shares After)
	refs    *refCache

	settings Settings

	// Independent version counters for cache-busting the display PNG endpoints.
	beforeVersion atomic.Int64
	afterVersion  atomic.Int64
	// generation increments on any change to the raw inputs or settings.
	generation atomic.Int64
}

// Batch returns the batch-mode store (many images of one scene).
func (s *Store) Batch() *BatchStore { return Batch }

// Global is the single shared store for the process.
var Global = &Store{settings: Settings{AutoRegister: true}}

// Snapshot is a consistent view of the raw inputs used to compute alignment.
type Snapshot struct {
	Before, After *image.NRGBA
	Baselines     []*image.NRGBA
	Generation    int64
	Settings      Settings
}

func (s *Store) invalidateLocked() {
	s.aligned, s.warped, s.refs = nil, nil, nil
	s.generation.Add(1)
}

// SetBefore stores the decoded before image and clears derived alignment data.
func (s *Store) SetBefore(img *image.NRGBA, dims Dims) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rawBefore, s.beforeDims = img, dims
	s.invalidateLocked()
	s.beforeVersion.Add(1)
}

// SetAfter stores the decoded after image and clears derived alignment data.
func (s *Store) SetAfter(img *image.NRGBA, dims Dims) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rawAfter, s.afterDims = img, dims
	s.invalidateLocked()
	s.afterVersion.Add(1)
}

// AddBaseline appends an extra before sweep and returns how many there are now.
func (s *Store) AddBaseline(img *image.NRGBA) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.baselines = append(s.baselines, img)
	s.invalidateLocked()
	return len(s.baselines)
}

// ClearBaselines removes all extra baselines.
func (s *Store) ClearBaselines() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.baselines) > 0 {
		s.baselines = nil
		s.invalidateLocked()
	}
}

// Configure changes registration settings and invalidates the aligned pair.
func (s *Store) Configure(st Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settings != st {
		s.settings = st
		s.invalidateLocked()
	}
}

// Snapshot returns the inputs needed to (re)compute alignment, or ok=false if
// either image is missing.
func (s *Store) Snapshot() (snap Snapshot, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.rawBefore == nil || s.rawAfter == nil {
		return Snapshot{}, false
	}
	return Snapshot{
		Before: s.rawBefore, After: s.rawAfter,
		Baselines:  append([]*image.NRGBA(nil), s.baselines...),
		Generation: s.generation.Load(), Settings: s.settings,
	}, true
}

// SetAligned stores a computed alignment, but only if the inputs it was
// derived from (identified by generation) are still current.
func (s *Store) SetAligned(generation int64, a imgproc.Aligned) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.generation.Load() != generation {
		return false
	}
	s.aligned, s.warped, s.refs = &a, nil, nil
	return true
}

// SetWarpedBefore stores a manual perspective-warped before image (analysis
// resolution) and its validity mask, if the snapshot is still current.
func (s *Store) SetWarpedBefore(generation int64, warped *image.NRGBA, valid *image.Gray) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.generation.Load() != generation || s.aligned == nil {
		return false
	}
	w := *s.aligned
	w.Before, w.Valid = warped, valid
	w.Reg = imgproc.Registration{Mode: "manual", Applied: true, Confidence: 1, Message: "manual alignment"}
	s.warped, s.refs = &w, nil
	return true
}

// ClearWarp removes the manual warp so analysis reverts to the automatic alignment.
func (s *Store) ClearWarp() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.warped, s.refs = nil, nil
}

// Analysis is what analyze handlers work from.
type Analysis struct {
	imgproc.Aligned
	Spread    *image.Gray // per-pixel baseline variability (nil with a single baseline)
	Baselines int         // number of baselines combined (1 = plain before/after)
}

type refCache struct {
	ref    *image.NRGBA
	spread *image.Gray
	n      int
}

// AnalysisPair returns the current alignment (manual warp if set), with the
// multi-baseline reference substituted for before when extra baselines exist.
// ok is false until both images are uploaded and aligned.
func (s *Store) AnalysisPair() (Analysis, bool) {
	s.mu.RLock()
	a := s.aligned
	if s.warped != nil {
		a = s.warped
	}
	if a == nil {
		s.mu.RUnlock()
		return Analysis{}, false
	}
	out := Analysis{Aligned: *a, Baselines: 1}
	if len(s.baselines) == 0 {
		s.mu.RUnlock()
		return out, true
	}
	cache, extras, gen, st := s.refs, append([]*image.NRGBA(nil), s.baselines...), s.generation.Load(), s.settings
	s.mu.RUnlock()

	if cache == nil {
		// Register each extra baseline to the analysis frame, then combine.
		set := []*image.NRGBA{a.Before}
		for _, e := range extras {
			r := imgproc.RegisterPairOpts(e, a.After, st.AutoRegister, st.LocalRefine)
			set = append(set, r.Before)
		}
		ref, spread := imgproc.CombineBaselines(set)
		cache = &refCache{ref: ref, spread: spread, n: len(set)}
		s.mu.Lock()
		if s.generation.Load() == gen {
			s.refs = cache
		}
		s.mu.Unlock()
	}
	out.Before, out.Spread, out.Baselines = cache.ref, cache.spread, cache.n
	return out, true
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

// Baselines returns the number of extra baselines stored.
func (s *Store) Baselines() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.baselines)
}

// Settings returns the current registration settings.
func (s *Store) Settings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// HasImages returns true once both images are uploaded and alignment is computed.
func (s *Store) HasImages() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.aligned != nil
}

func (s *Store) BeforeVersion() int64 { return s.beforeVersion.Load() }
func (s *Store) AfterVersion() int64  { return s.afterVersion.Load() }

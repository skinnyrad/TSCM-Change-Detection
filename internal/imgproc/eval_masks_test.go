package imgproc

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

// TestEvalMasks scores the pipeline, with the UI's default settings, against a
// pixel-labelled change dataset laid out as t0/ (before), t1/ (after) and
// mask/ (white = changed), e.g. TSUNAMI or VL-CMU-CD. Skipped unless MASK_DIR
// is set. CFG overrides the defaults with the same keys as TestEvalLab plus
// open, close, minreg, blur (×10).
//
// These are outdoor scenes captured months or years apart, with labels for
// structural change only, so seasonal and lighting differences count as false
// positives. Use them to measure false-positive control, not as a proxy for
// indoor sweep accuracy.
//
//	MASK_DIR=tmp/datasets/TSUNAMI go test ./internal/imgproc -run TestEvalMasks -v
func TestEvalMasks(t *testing.T) {
	root := os.Getenv("MASK_DIR")
	if root == "" {
		t.Skip("MASK_DIR not set")
	}
	cfg, opts := evalOpts(os.Getenv("CFG"))
	t.Logf("cfg=%v", cfg)

	root, _ = filepath.Abs(root)
	names, _ := filepath.Glob(filepath.Join(root, "mask", "*.png"))
	sort.Strings(names)
	type score struct{ tp, fp, fn, tn int }
	scores := make([]score, len(names))
	flagged := make([]float64, len(names))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i, mp := range names {
		wg.Add(1)
		go func(i int, mp string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			stem := strings.TrimSuffix(filepath.Base(mp), filepath.Ext(mp))
			find := func(dir string) string {
				m, _ := filepath.Glob(filepath.Join(root, dir, stem+".*"))
				if len(m) == 0 {
					return ""
				}
				return m[0]
			}
			b, err1 := loadNRGBA(find("t0"))
			a, err2 := loadNRGBA(find("t1"))
			gt, err3 := loadNRGBA(mp)
			if err1 != nil || err2 != nil || err3 != nil {
				t.Errorf("%s: %v %v %v", stem, err1, err2, err3)
				return
			}
			al := RegisterPairOpts(b, a, cfg["reg"] > 0, cfg["local"] > 0)
			o := opts
			o.Valid = al.Valid
			mask := ComputeDiffV2(al.Before, al.After, o).Mask
			mw, mh := mask.Bounds().Dx(), mask.Bounds().Dy()
			gw, gh := gt.Bounds().Dx(), gt.Bounds().Dy()
			var s score
			changed := 0
			for y := 0; y < gh; y++ {
				for x := 0; x < gw; x++ {
					p := mask.Pix[(y*mh/gh)*mask.Stride+x*mw/gw] > 0
					g := gt.Pix[y*gt.Stride+x*4] >= 128
					switch {
					case p && g:
						s.tp++
					case p:
						s.fp++
					case g:
						s.fn++
					default:
						s.tn++
					}
					if p {
						changed++
					}
				}
			}
			scores[i] = s
			flagged[i] = float64(changed) / float64(gw*gh)
		}(i, mp)
	}
	wg.Wait()

	var tot score
	var meanFlag, meanF1 float64
	for i, s := range scores {
		tot.tp, tot.fp, tot.fn, tot.tn = tot.tp+s.tp, tot.fp+s.fp, tot.fn+s.fn, tot.tn+s.tn
		meanFlag += flagged[i]
		meanF1 += f1(s.tp, s.fp, s.fn)
	}
	n := float64(len(scores))
	p := float64(tot.tp) / float64(max(tot.tp+tot.fp, 1))
	r := float64(tot.tp) / float64(max(tot.tp+tot.fn, 1))
	gtFrac := float64(tot.tp+tot.fn) / float64(max(tot.tp+tot.fp+tot.fn+tot.tn, 1))
	t.Logf("pairs=%d precision=%.3f recall=%.3f F1=%.3f IoU=%.3f meanPairF1=%.3f flagged=%.1f%% (labelled %.1f%%)",
		len(scores), p, r, f1(tot.tp, tot.fp, tot.fn), float64(tot.tp)/float64(max(tot.tp+tot.fp+tot.fn, 1)),
		meanF1/n, 100*meanFlag/n, 100*gtFrac)
}

func f1(tp, fp, fn int) float64 {
	if tp == 0 {
		return 0
	}
	return 2 * float64(tp) / float64(2*tp+fp+fn)
}

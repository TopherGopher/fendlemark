package fri

import (
	"errors"
	"math"
	"testing"
)

// breath synthesises a plume series: onset, peak, exponential decay.
func breath(frames int, onset, decay, noise float64, seed int) []float64 {
	out := make([]float64, frames)
	r := uint32(seed*2654435761 + 1)
	for i := range out {
		t := float64(i) / float64(frames)
		r = r*1664525 + 1013904223
		n := (float64(r>>8)/float64(1<<24) - 0.5) * noise
		if t < onset {
			out[i] = t/onset + n
		} else {
			out[i] = math.Exp(-(t-onset)*decay) + n
		}
	}
	return out
}

func TestEnrolAndMatch(t *testing.T) {
	var samples []Signature
	for i := 0; i < 3; i++ {
		s, err := Fit(breath(240, 0.12, 6, 0.02, i))
		if err != nil {
			t.Fatal(err)
		}
		samples = append(samples, s)
	}
	e, err := Enrol("marit@example.com", Keeper, samples)
	if err != nil {
		t.Fatal(err)
	}
	same, _ := Fit(breath(240, 0.12, 6, 0.02, 9))
	if score, ok := Match(same, e, 0.62); !ok {
		t.Fatalf("same breather should match; score %.3f", score)
	}
	other, _ := Fit(breath(240, 0.45, 1.2, 0.02, 3))
	if score, ok := Match(other, e, 0.95); ok {
		t.Fatalf("a different plume should not match at high sensitivity; score %.3f", score)
	}
}

func TestTriadRequired(t *testing.T) {
	s, _ := Fit(breath(100, 0.1, 5, 0, 1))
	if _, err := Enrol("x", Viewer, []Signature{s, s}); !errors.Is(err, ErrTriadIncomplete) {
		t.Fatalf("want FRI-0019, got %v", err)
	}
}

func TestNotRecognised(t *testing.T) {
	if _, err := Fit([]float64{0, 0, 0, 0, 0, 0, 0, 0, 0}); !errors.Is(err, ErrNotRecognised) {
		t.Fatalf("want FRI-0042, got %v", err)
	}
}

func TestClassify(t *testing.T) {
	nasal, _ := Fit(breath(240, 0.3, 1.5, 0, 1))
	mouth, _ := Fit(breath(240, 0.05, 9, 0, 1))
	if Classify(nasal) == Mouth {
		t.Fatal("slow, long plume classified as mouth")
	}
	if Classify(mouth) == Nasal {
		t.Fatal("fast, short plume classified as nasal")
	}
}

func TestFileStore(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := Fit(breath(100, 0.1, 5, 0, 1))
	e, _ := Enrol("dov@example.com", Operator, []Signature{s, s, s})
	if err := fs.Put(e); err != nil {
		t.Fatal(err)
	}
	fs2, _ := NewFileStore(dir)
	got, err := fs2.Get("dov@example.com")
	if err != nil || got.Role != Operator {
		t.Fatalf("round trip failed: %v %+v", err, got)
	}
}

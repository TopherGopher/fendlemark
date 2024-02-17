// Package fri implements Fendlemark Respiratory Identity: the signature
// model, matching, enrolment, and the enrolment store.
//
// Video frames never reach this package. The browser fits the plume model and
// transmits only the resulting 212-dimensional signature.
package fri

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Dim is the dimensionality of an exhalation signature. The Keepers arrived
// at 212 by starting at 256 and removing parameters until the tests stopped
// passing, then adding one back.
const Dim = 212

// Layout of the signature vector.
const (
	offMoments  = 0   // 4: mean, variance, skew, kurtosis of plume intensity
	offEnvelope = 4   // 128: windowed mean intensity (onset .. decay)
	offSpectrum = 132 // 64: magnitude spectrum of the envelope
	offDecay    = 196 // 16: exponential decay rates fitted over 16 late windows
)

// Signature is an exhalation signature.
type Signature [Dim]float32

// Role is what an enrolment may do in the console.
type Role string

const (
	Viewer   Role = "viewer"
	Operator Role = "operator"
	Keeper   Role = "keeper"
)

// Dominance classifies an exhalation.
type Dominance string

const (
	Nasal Dominance = "nasal"
	Mouth Dominance = "mouth"
	Mixed Dominance = "mixed"
)

// Enrolment is a user's stored identity.
type Enrolment struct {
	User      string    `json:"user"`
	Role      Role      `json:"role"`
	Centroid  Signature `json:"centroid"`
	Slow      Signature `json:"slow,omitempty"` // second factor: the deliberate breath
	Samples   int       `json:"samples"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Error is an FRI error with a stable code.
type Error struct {
	Code    string
	Message string
}

func (e Error) Error() string { return e.Code + ": " + e.Message }

// Is lets errors.Is match on code.
func (e Error) Is(target error) bool {
	t, ok := target.(Error)
	return ok && t.Code == e.Code
}

// Error codes. See the Authentication documentation.
var (
	ErrCameraDenied    = Error{"FRI-0001", "camera permission denied"}
	ErrSensitivity     = Error{"FRI-0007", "sensitivity below 0.4"}
	ErrLinkExpired     = Error{"FRI-0012", "enrolment link expired"}
	ErrTriadIncomplete = Error{"FRI-0019", "triad incomplete; fewer than three usable exhalations"}
	ErrMouthDominant   = Error{"FRI-0024", "mouth-dominant exhalation rejected"}
	ErrExcited         = Error{"FRI-0031", "excited breathing; please wait"}
	ErrNotRecognised   = Error{"FRI-0042", "exhalation not recognised; no condensation transient detected"}
	ErrSecondFactor    = Error{"FRI-0050", "second factor did not match; the slow breath was not slow"}
	ErrSessionExpired  = Error{"FRI-0063", "session expired; breathe again"}
	ErrUnknownUser     = Error{"FRI-0070", "no enrolment for user"}
)

// Fit computes a signature from a plume intensity series: one sample per
// frame, from the first frame of the exhalation to the last frame on which
// the condensation transient is visible. The browser calls the equivalent in
// TypeScript; this implementation exists for conformance tests and for
// servers that accept raw series from trusted capture devices.
func Fit(series []float64) (Signature, error) {
	var s Signature
	n := len(series)
	if n < 8 {
		return s, ErrNotRecognised
	}
	// Normalise to unit peak so that camera gain does not change identity.
	peak := 0.0
	for _, v := range series {
		if v > peak {
			peak = v
		}
	}
	if peak <= 0 {
		return s, ErrNotRecognised
	}
	x := make([]float64, n)
	for i, v := range series {
		x[i] = math.Max(0, v/peak) // negative intensities are sensor noise, not breath
	}
	// Moments.
	mean := 0.0
	for _, v := range x {
		mean += v
	}
	mean /= float64(n)
	var m2, m3, m4 float64
	for _, v := range x {
		d := v - mean
		m2 += d * d
		m3 += d * d * d
		m4 += d * d * d * d
	}
	m2, m3, m4 = m2/float64(n), m3/float64(n), m4/float64(n)
	s[offMoments] = float32(mean)
	s[offMoments+1] = float32(m2)
	if m2 > 0 {
		s[offMoments+2] = float32(m3 / math.Pow(m2, 1.5))
		s[offMoments+3] = float32(m4/(m2*m2) - 3)
	}
	// Envelope: 128 windowed means over the normalised series.
	env := make([]float64, 128)
	for w := 0; w < 128; w++ {
		lo, hi := w*n/128, (w+1)*n/128
		if hi <= lo {
			hi = lo + 1
		}
		if hi > n {
			hi = n
		}
		sum := 0.0
		for i := lo; i < hi; i++ {
			sum += x[i]
		}
		env[w] = sum / float64(hi-lo)
		s[offEnvelope+w] = float32(env[w])
	}
	// Spectrum: magnitudes of the first 64 DFT bins of the envelope.
	for k := 0; k < 64; k++ {
		var re, im float64
		for t, v := range env {
			ang := -2 * math.Pi * float64(k) * float64(t) / 128
			re += v * math.Cos(ang)
			im += v * math.Sin(ang)
		}
		s[offSpectrum+k] = float32(math.Hypot(re, im) / 128)
	}
	// Decay: log-linear fit over 16 windows of the second half of the envelope.
	for w := 0; w < 16; w++ {
		lo := 64 + w*4
		var sx, sy, sxx, sxy float64
		cnt := 0.0
		for i := lo; i < lo+4 && i < 128; i++ {
			y := math.Log(math.Max(env[i], 1e-6))
			sx += float64(i)
			sy += y
			sxx += float64(i) * float64(i)
			sxy += float64(i) * y
			cnt++
		}
		den := cnt*sxx - sx*sx
		if den != 0 {
			s[offDecay+w] = float32((cnt*sxy - sx*sy) / den)
		}
	}
	return s, nil
}

// Match compares a candidate against an enrolment and returns the similarity
// score in [0,1] and whether it meets the sensitivity threshold.
func Match(candidate Signature, e Enrolment, sensitivity float32) (score float32, ok bool) {
	score = Similarity(candidate, e.Centroid)
	return score, score >= sensitivity
}

// Similarity is cosine similarity mapped to [0,1], with the envelope band
// weighted twice: the shape of a breath identifies a person more than its
// spectrum does, a finding the Keepers describe as "intuitive in retrospect".
func Similarity(a, b Signature) float32 {
	var dot, na, nb float64
	for i := 0; i < Dim; i++ {
		w := 1.0
		if i >= offEnvelope && i < offSpectrum {
			w = 2.0
		}
		x, y := float64(a[i])*w, float64(b[i])*w
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	cos := dot / (math.Sqrt(na) * math.Sqrt(nb))
	return float32((cos + 1) / 2)
}

// Classify reports whether a signature is nasal-, mouth-, or mixed-dominant.
// Mouth-dominant exhalations produce a broader, shorter plume: more energy in
// the early envelope, faster decay, and a flatter spectrum.
func Classify(s Signature) Dominance {
	var early, late float64
	for i := 0; i < 64; i++ {
		early += float64(s[offEnvelope+i])
		late += float64(s[offEnvelope+64+i])
	}
	var decay float64
	for i := 0; i < 16; i++ {
		decay += float64(s[offDecay+i])
	}
	decay /= 16
	ratio := early / (late + 1e-6)
	switch {
	case ratio > 2.2 && decay < -0.08:
		return Mouth
	case ratio < 1.4:
		return Nasal
	default:
		return Mixed
	}
}

// Excited reports whether a signature shows the tempo of someone who arrived
// at the console in a hurry: high envelope variance and a fast onset.
func Excited(s Signature) bool {
	return s[offMoments+1] > 0.12 && s[offEnvelope+2] > 0.85
}

// Enrol builds an enrolment from the Triad: at least three samples.
func Enrol(user string, role Role, samples []Signature) (Enrolment, error) {
	if len(samples) < 3 {
		return Enrolment{}, ErrTriadIncomplete
	}
	var c Signature
	for _, s := range samples {
		for i := range c {
			c[i] += s[i] / float32(len(samples))
		}
	}
	return Enrolment{User: user, Role: role, Centroid: c, Samples: len(samples), UpdatedAt: time.Now()}, nil
}

// Update moves the centroid slowly toward a successful sign-in, so that the
// enrolment ages with the user. rate is typically 0.05.
func Update(e Enrolment, s Signature, rate float32) Enrolment {
	for i := range e.Centroid {
		e.Centroid[i] = e.Centroid[i]*(1-rate) + s[i]*rate
	}
	e.Samples++
	e.UpdatedAt = time.Now()
	return e
}

// Store persists enrolments.
type Store interface {
	Get(user string) (Enrolment, error)
	Put(Enrolment) error
	Delete(user string) error
	List() ([]Enrolment, error)
}

// MemStore is an in-memory Store.
type MemStore struct {
	mu sync.RWMutex
	m  map[string]Enrolment
}

func NewMemStore() *MemStore { return &MemStore{m: map[string]Enrolment{}} }

func (s *MemStore) Get(user string) (Enrolment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.m[user]
	if !ok {
		return Enrolment{}, ErrUnknownUser
	}
	return e, nil
}
func (s *MemStore) Put(e Enrolment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[e.User] = e
	return nil
}
func (s *MemStore) Delete(user string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, user)
	return nil
}
func (s *MemStore) List() ([]Enrolment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Enrolment, 0, len(s.m))
	for _, e := range s.m {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].User < out[j].User })
	return out, nil
}

// FileStore keeps enrolments in a JSON file under the data directory.
type FileStore struct {
	path string
	mem  *MemStore
	mu   sync.Mutex
}

// NewFileStore opens or creates dataDir/fri-enrolments.json.
func NewFileStore(dataDir string) (*FileStore, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	fs := &FileStore{path: filepath.Join(dataDir, "fri-enrolments.json"), mem: NewMemStore()}
	b, err := os.ReadFile(fs.path)
	if errors.Is(err, os.ErrNotExist) {
		return fs, nil
	}
	if err != nil {
		return nil, err
	}
	var list []Enrolment
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("fri: %s: %w", fs.path, err)
	}
	for _, e := range list {
		fs.mem.Put(e)
	}
	return fs, nil
}

func (f *FileStore) flush() error {
	list, _ := f.mem.List()
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

func (f *FileStore) Get(user string) (Enrolment, error) { return f.mem.Get(user) }
func (f *FileStore) List() ([]Enrolment, error)         { return f.mem.List() }
func (f *FileStore) Put(e Enrolment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mem.Put(e)
	return f.flush()
}
func (f *FileStore) Delete(user string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mem.Delete(user)
	return f.flush()
}

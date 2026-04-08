package ops

import (
	"math"
	"math/big"
	"math/rand"
	"time"
)

// DelayMode defines the delay distribution type.
type DelayMode int

const (
	DelayModeUniform  DelayMode = iota // classic uniform random
	DelayModeGaussian                  // gaussian/normal distribution
)

// DelayConfig holds configuration for random delays between transactions.
type DelayConfig struct {
	Enabled bool
	MinMs   int // minimum delay in milliseconds
	MaxMs   int // maximum delay in milliseconds
	Mode    DelayMode
}

// DefaultDelay returns a sensible default delay config (3-12 seconds).
func DefaultDelay() DelayConfig {
	return DelayConfig{
		Enabled: true,
		MinMs:   3000,
		MaxMs:   12000,
		Mode:    DelayModeUniform,
	}
}

// NoDelay returns a disabled delay config.
func NoDelay() DelayConfig {
	return DelayConfig{Enabled: false}
}

// Wait pauses for a random duration between MinMs and MaxMs.
// Returns the actual duration waited.
func (d DelayConfig) Wait() time.Duration {
	if !d.Enabled || d.MaxMs <= 0 {
		return 0
	}

	min := d.MinMs
	max := d.MaxMs
	if min > max {
		min, max = max, min
	}
	if min == max {
		dur := time.Duration(min) * time.Millisecond
		time.Sleep(dur)
		return dur
	}

	var ms int
	switch d.Mode {
	case DelayModeGaussian:
		ms = gaussianRange(min, max)
	default:
		ms = min + rand.Intn(max-min)
	}

	dur := time.Duration(ms) * time.Millisecond
	time.Sleep(dur)
	return dur
}

// Jitter adds a small random jitter (0-500ms) on top of the base delay.
// Useful for concurrent operations to avoid thundering herd.
func (d DelayConfig) Jitter() time.Duration {
	if !d.Enabled {
		return 0
	}
	ms := rand.Intn(500)
	dur := time.Duration(ms) * time.Millisecond
	time.Sleep(dur)
	return dur
}

// gaussianRange generates a value in [min, max] using Box-Muller gaussian.
// Mean is centered, 99.7% of values fall within the range.
func gaussianRange(min, max int) int {
	mean := float64(min+max) / 2.0
	stddev := float64(max-min) / 6.0 // 3σ = half range → 99.7% within [min,max]

	// Box-Muller transform
	u1 := rand.Float64()
	u2 := rand.Float64()
	z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)

	val := mean + z*stddev
	// Clamp to range
	if val < float64(min) {
		val = float64(min)
	}
	if val > float64(max) {
		val = float64(max)
	}
	return int(val)
}

// RandomizeAmount adds ±variance% to a given amount for anti-pattern detection.
// variancePct is 0-100 (e.g., 10 = ±10%).
func RandomizeAmount(amount *big.Int, variancePct int) *big.Int {
	if amount == nil || variancePct <= 0 {
		return amount
	}
	if variancePct > 50 {
		variancePct = 50
	}

	// Generate random factor between (100-variance)% and (100+variance)%
	minPct := 100 - variancePct
	maxPct := 100 + variancePct
	pct := minPct + rand.Intn(maxPct-minPct+1)

	result := new(big.Int).Mul(amount, big.NewInt(int64(pct)))
	result.Div(result, big.NewInt(100))
	return result
}

// ModeName returns a display string for the delay mode.
func (d DelayConfig) ModeName() string {
	if !d.Enabled {
		return "OFF"
	}
	switch d.Mode {
	case DelayModeGaussian:
		return "Gaussian"
	default:
		return "Uniform"
	}
}

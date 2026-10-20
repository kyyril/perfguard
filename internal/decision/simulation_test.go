package decision_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/decision"
	"github.com/kyyril/perfguard/internal/model"
)

// buildRun constructs a model.Run containing a single benchmark with the specified metric values.
func buildRun(id, benchName, metric string, values []float64) model.Run {
	samples := make([]model.Sample, len(values))
	for i, v := range values {
		samples[i] = model.Sample{
			Values: map[string]float64{metric: v},
		}
	}
	return model.Run{
		ID: id,
		Benchmarks: []model.BenchmarkResult{
			{
				Name:    benchName,
				Samples: samples,
			},
		},
	}
}

func runTrial(cfg *config.Config, bVals, hVals []float64) (model.Status, string, error) {
	bRun := buildRun("base", "BenchmarkSim", "sec/op", bVals)
	hRun := buildRun("head", "BenchmarkSim", "sec/op", hVals)

	cmp, err := decision.CompareRuns(bRun, hRun, cfg)
	if err != nil {
		return "", "", err
	}
	if len(cmp.Items) == 0 {
		return "", "", fmt.Errorf("no comparison items generated")
	}
	return cmp.Items[0].Status, cmp.Items[0].Reason, nil
}

// TestSimulation verifies false positive rate and detection power across synthetic distributions
// as specified in docs/TESTING.md section 3.1.
func TestSimulation(t *testing.T) {
	t.Parallel()

	const trials = 2000
	const n = 10

	cfg := config.Default()
	cfg.Benchmarks.Count = n
	cfg.Thresholds.MinSamples = 6
	cfg.Thresholds.Alpha = 0.05
	cfg.Noise.MaxCV = 10.0
	cfg.Noise.OnNoisy = "inconclusive"

	t.Run("Stable_AA", func(t *testing.T) {
		t.Parallel()
		//nolint:gosec // Fixed seed required for deterministic statistical simulations.
		rng := rand.New(rand.NewSource(1001))

		var failCount, warnCount int
		for i := 0; i < trials; i++ {
			bVals := make([]float64, n)
			hVals := make([]float64, n)
			for j := 0; j < n; j++ {
				bVals[j] = 100.0 + rng.NormFloat64()*2.0
				hVals[j] = 100.0 + rng.NormFloat64()*2.0
			}

			st, _, err := runTrial(cfg, bVals, hVals)
			if err != nil {
				t.Fatalf("trial %d failed: %v", i, err)
			}
			if st == model.StatusFail {
				failCount++
			}
			if st == model.StatusWarn {
				warnCount++
			}
		}

		failRate := float64(failCount) / float64(trials)
		warnRate := float64(warnCount) / float64(trials)

		// Assert: fail at most 1%, warn at most 5%
		if failRate > 0.01 {
			t.Errorf("Stable A/A fail rate too high: got %.2f%% (max 1.0%%, count %d)", failRate*100, failCount)
		}
		if warnRate > 0.05 {
			t.Errorf("Stable A/A warn rate too high: got %.2f%% (max 5.0%%, count %d)", warnRate*100, warnCount)
		}
	})

	t.Run("HeavyTailed_AA", func(t *testing.T) {
		t.Parallel()
		//nolint:gosec // Fixed seed required for deterministic statistical simulations.
		rng := rand.New(rand.NewSource(1002))

		// Lognormal distribution: log(X) ~ N(mu, sigma).
		// With sigma=0.03, CV is approx 3%, well below the noise cutoff.
		const sigma = 0.03
		mu := math.Log(100.0) - (sigma*sigma)/2.0

		var failCount, warnCount int
		for i := 0; i < trials; i++ {
			bVals := make([]float64, n)
			hVals := make([]float64, n)
			for j := 0; j < n; j++ {
				bVals[j] = math.Exp(mu + rng.NormFloat64()*sigma)
				hVals[j] = math.Exp(mu + rng.NormFloat64()*sigma)
			}

			st, _, err := runTrial(cfg, bVals, hVals)
			if err != nil {
				t.Fatalf("trial %d failed: %v", i, err)
			}
			if st == model.StatusFail {
				failCount++
			}
			if st == model.StatusWarn {
				warnCount++
			}
		}

		failRate := float64(failCount) / float64(trials)
		warnRate := float64(warnCount) / float64(trials)

		if failRate > 0.01 {
			t.Errorf("Heavy-tailed A/A fail rate too high: got %.2f%% (max 1.0%%, count %d)", failRate*100, failCount)
		}
		if warnRate > 0.05 {
			t.Errorf("Heavy-tailed A/A warn rate too high: got %.2f%% (max 5.0%%, count %d)", warnRate*100, warnCount)
		}
	})

	t.Run("Bimodal_AA", func(t *testing.T) {
		t.Parallel()
		//nolint:gosec // Fixed seed required for deterministic statistical simulations.
		rng := rand.New(rand.NewSource(1003))

		// Two modes separated by 5%: Mode 1 at 97.5, Mode 2 at 102.5.
		var failCount, warnCount int
		for i := 0; i < trials; i++ {
			bVals := make([]float64, n)
			hVals := make([]float64, n)
			for j := 0; j < n; j++ {
				if rng.Float64() < 0.5 {
					bVals[j] = 97.5 + rng.NormFloat64()*1.0
				} else {
					bVals[j] = 102.5 + rng.NormFloat64()*1.0
				}

				if rng.Float64() < 0.5 {
					hVals[j] = 97.5 + rng.NormFloat64()*1.0
				} else {
					hVals[j] = 102.5 + rng.NormFloat64()*1.0
				}
			}

			st, _, err := runTrial(cfg, bVals, hVals)
			if err != nil {
				t.Fatalf("trial %d failed: %v", i, err)
			}
			if st == model.StatusFail {
				failCount++
			}
			if st == model.StatusWarn {
				warnCount++
			}
		}

		failRate := float64(failCount) / float64(trials)
		warnRate := float64(warnCount) / float64(trials)

		if failRate > 0.01 {
			t.Errorf("Bimodal A/A fail rate too high: got %.2f%% (max 1.0%%, count %d)", failRate*100, failCount)
		}
		if warnRate > 0.05 {
			t.Errorf("Bimodal A/A warn rate too high: got %.2f%% (max 5.0%%, count %d)", warnRate*100, warnCount)
		}
	})

	t.Run("Drift_AA", func(t *testing.T) {
		t.Parallel()
		//nolint:gosec // Fixed seed required for deterministic statistical simulations.
		rng := rand.New(rand.NewSource(1004))

		// Linear trend across the run simulating machine heating or background drift.
		var failCount, warnCount int
		for i := 0; i < trials; i++ {
			bVals := make([]float64, n)
			hVals := make([]float64, n)
			for j := 0; j < n; j++ {
				trend := float64(j) * 0.2
				bVals[j] = 100.0 + trend + rng.NormFloat64()*2.0
				hVals[j] = 100.0 + trend + rng.NormFloat64()*2.0
			}

			st, _, err := runTrial(cfg, bVals, hVals)
			if err != nil {
				t.Fatalf("trial %d failed: %v", i, err)
			}
			if st == model.StatusFail {
				failCount++
			}
			if st == model.StatusWarn {
				warnCount++
			}
		}

		failRate := float64(failCount) / float64(trials)
		warnRate := float64(warnCount) / float64(trials)

		if failRate > 0.01 {
			t.Errorf("Drift A/A fail rate too high: got %.2f%% (max 1.0%%, count %d)", failRate*100, failCount)
		}
		if warnRate > 0.05 {
			t.Errorf("Drift A/A warn rate too high: got %.2f%% (max 5.0%%, count %d)", warnRate*100, warnCount)
		}
	})

	t.Run("Shift_2x_Warn", func(t *testing.T) {
		t.Parallel()
		//nolint:gosec // Fixed seed required for deterministic statistical simulations.
		rng := rand.New(rand.NewSource(1005))

		// Shift 2x warn threshold (warn is 5%, 2x is +10% shift).
		// Base: Normal(100, 3) (CV 3%). Head: Normal(110, 3.3).
		var detectedCount int
		for i := 0; i < trials; i++ {
			bVals := make([]float64, n)
			hVals := make([]float64, n)
			for j := 0; j < n; j++ {
				bVals[j] = 100.0 + rng.NormFloat64()*3.0
				hVals[j] = 110.0 + rng.NormFloat64()*3.3
			}

			st, _, err := runTrial(cfg, bVals, hVals)
			if err != nil {
				t.Fatalf("trial %d failed: %v", i, err)
			}
			if st == model.StatusWarn || st == model.StatusFail {
				detectedCount++
			}
		}

		power := float64(detectedCount) / float64(trials)
		// Assert: warn or fail in 90% or more of trials
		if power < 0.90 {
			t.Errorf("Detection power too low for 2x warn shift: got %.2f%% (min 90.0%%, count %d)", power*100, detectedCount)
		}
	})

	t.Run("Below_Threshold_Shift", func(t *testing.T) {
		t.Parallel()
		//nolint:gosec // Fixed seed required for deterministic statistical simulations.
		rng := rand.New(rand.NewSource(1006))

		// 3% delta with warn threshold at 5%. Fail threshold is 15%.
		// Slower by 3%, but should not trigger a false fail.
		var failCount int
		for i := 0; i < trials; i++ {
			bVals := make([]float64, n)
			hVals := make([]float64, n)
			for j := 0; j < n; j++ {
				bVals[j] = 100.0 + rng.NormFloat64()*2.0
				hVals[j] = 103.0 + rng.NormFloat64()*2.0
			}

			st, _, err := runTrial(cfg, bVals, hVals)
			if err != nil {
				t.Fatalf("trial %d failed: %v", i, err)
			}
			if st == model.StatusFail {
				failCount++
			}
		}

		failRate := float64(failCount) / float64(trials)
		// Assert: fail at most 1%
		if failRate > 0.01 {
			t.Errorf("Below-threshold shift fail rate too high: got %.2f%% (max 1.0%%, count %d)", failRate*100, failCount)
		}
	})

	t.Run("High_Noise", func(t *testing.T) {
		t.Parallel()
		//nolint:gosec // Fixed seed required for deterministic statistical simulations.
		rng := rand.New(rand.NewSource(1007))

		// Normal distribution with CV ~20% (mu=100, sigma=20).
		// MaxCV threshold is 10%, so majority should be flagged as inconclusive.
		var inconclusiveCount, failCount int
		for i := 0; i < trials; i++ {
			bVals := make([]float64, n)
			hVals := make([]float64, n)
			for j := 0; j < n; j++ {
				bVals[j] = 100.0 + rng.NormFloat64()*20.0
				hVals[j] = 100.0 + rng.NormFloat64()*20.0
			}

			st, _, err := runTrial(cfg, bVals, hVals)
			if err != nil {
				t.Fatalf("trial %d failed: %v", i, err)
			}
			if st == model.StatusInconclusive {
				inconclusiveCount++
			}
			if st == model.StatusFail {
				failCount++
			}
		}

		inconclusiveRate := float64(inconclusiveCount) / float64(trials)
		failRate := float64(failCount) / float64(trials)

		// Assert: majority inconclusive (> 50%), fail at most 1%
		if inconclusiveRate <= 0.50 {
			t.Errorf("High noise inconclusive rate too low: got %.2f%% (min 50.0%%, count %d)", inconclusiveRate*100, inconclusiveCount)
		}
		if failRate > 0.01 {
			t.Errorf("High noise fail rate too high: got %.2f%% (max 1.0%%, count %d)", failRate*100, failCount)
		}
	})
}

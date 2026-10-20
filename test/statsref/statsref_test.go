package statsref_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyyril/perfguard/internal/stats"
)

type ReferenceCase struct {
	Name       string    `json:"name"`
	Base       []float64 `json:"base"`
	Head       []float64 `json:"head"`
	BaseMedian float64   `json:"base_median"`
	HeadMedian float64   `json:"head_median"`
	BaseMean   float64   `json:"base_mean"`
	HeadMean   float64   `json:"head_mean"`
	BaseStddev float64   `json:"base_stddev"`
	HeadStddev float64   `json:"head_stddev"`
	BaseCV     float64   `json:"base_cv"`
	HeadCV     float64   `json:"head_cv"`
	DeltaPct   float64   `json:"delta_pct"`
	UStatistic float64   `json:"u_statistic"`
	PValue     float64   `json:"p_value"`
}

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestStatsAgainstReferenceJSON(t *testing.T) {
	t.Parallel()

	refPath, err := filepath.Abs("../../testdata/stats/reference.json")
	if err != nil {
		t.Fatalf("failed to resolve path to reference.json: %v", err)
	}

	data, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("failed reading reference.json: %v", err)
	}

	var cases []ReferenceCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed unmarshaling reference cases: %v", err)
	}

	if len(cases) == 0 {
		t.Fatal("reference.json contains no test cases")
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			bMed, err := stats.Median(tc.Base)
			if err != nil {
				t.Fatalf("unexpected error on Base Median: %v", err)
			}
			if !almostEqual(bMed, tc.BaseMedian, 1e-6) {
				t.Errorf("base median mismatch: got %v, expected %v", bMed, tc.BaseMedian)
			}

			hMed, err := stats.Median(tc.Head)
			if err != nil {
				t.Fatalf("unexpected error on Head Median: %v", err)
			}
			if !almostEqual(hMed, tc.HeadMedian, 1e-6) {
				t.Errorf("head median mismatch: got %v, expected %v", hMed, tc.HeadMedian)
			}

			bMean, err := stats.Mean(tc.Base)
			if err != nil {
				t.Fatalf("unexpected error on Base Mean: %v", err)
			}
			if !almostEqual(bMean, tc.BaseMean, 1e-6) {
				t.Errorf("base mean mismatch: got %v, expected %v", bMean, tc.BaseMean)
			}

			hMean, err := stats.Mean(tc.Head)
			if err != nil {
				t.Fatalf("unexpected error on Head Mean: %v", err)
			}
			if !almostEqual(hMean, tc.HeadMean, 1e-6) {
				t.Errorf("head mean mismatch: got %v, expected %v", hMean, tc.HeadMean)
			}

			bStd, err := stats.StdDev(tc.Base)
			if err != nil {
				t.Fatalf("unexpected error on Base StdDev: %v", err)
			}
			if !almostEqual(bStd, tc.BaseStddev, 1e-5) {
				t.Errorf("base stddev mismatch: got %v, expected %v", bStd, tc.BaseStddev)
			}

			hStd, err := stats.StdDev(tc.Head)
			if err != nil {
				t.Fatalf("unexpected error on Head StdDev: %v", err)
			}
			if !almostEqual(hStd, tc.HeadStddev, 1e-5) {
				t.Errorf("head stddev mismatch: got %v, expected %v", hStd, tc.HeadStddev)
			}

			bCV, err := stats.CV(tc.Base)
			if err != nil {
				t.Fatalf("unexpected error on Base CV: %v", err)
			}
			if !almostEqual(bCV, tc.BaseCV, 1e-4) {
				t.Errorf("base CV mismatch: got %v, expected %v", bCV, tc.BaseCV)
			}

			hCV, err := stats.CV(tc.Head)
			if err != nil {
				t.Fatalf("unexpected error on Head CV: %v", err)
			}
			if !almostEqual(hCV, tc.HeadCV, 1e-4) {
				t.Errorf("head CV mismatch: got %v, expected %v", hCV, tc.HeadCV)
			}

			delta, err := stats.DeltaPct(bMed, hMed)
			if err != nil {
				t.Fatalf("unexpected error on DeltaPct: %v", err)
			}
			if !almostEqual(delta, tc.DeltaPct, 1e-4) {
				t.Errorf("delta_pct mismatch: got %v, expected %v", delta, tc.DeltaPct)
			}

			u, p, err := stats.MannWhitneyU(tc.Base, tc.Head)
			if err != nil {
				t.Fatalf("unexpected error on MannWhitneyU: %v", err)
			}
			if !almostEqual(u, tc.UStatistic, 1e-4) {
				t.Errorf("U statistic mismatch: got %v, expected %v", u, tc.UStatistic)
			}
			if !almostEqual(p, tc.PValue, 0.03) {
				t.Errorf("P value mismatch: got %v, expected %v", p, tc.PValue)
			}
		})
	}
}

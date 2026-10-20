// Package stats provides pure, deterministic statistical routines
// for benchmark comparisons, including median, CV, Mann-Whitney U tests,
// and bootstrap confidence intervals.
package stats

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"

	moremath "github.com/aclements/go-moremath/stats"
)

var (
	// ErrEmptyInput indicates that a required numeric slice was empty.
	ErrEmptyInput = errors.New("stats: empty input slice")

	// ErrInvalidValue indicates that a slice contains NaN or infinite values.
	ErrInvalidValue = errors.New("stats: input contains NaN or Inf")

	// ErrZeroMean indicates that the mean is zero when calculating coefficient of variation.
	ErrZeroMean = errors.New("stats: zero mean with non-zero dispersion")

	// ErrZeroBase indicates that base median is zero and percentage change cannot be computed.
	ErrZeroBase = errors.New("stats: base median is zero")

	// ErrInvalidConfidence indicates confidence level is outside (0, 1).
	ErrInvalidConfidence = errors.New("stats: confidence level must be between 0 and 1")
)

func validateSlice(xs []float64) error {
	if len(xs) == 0 {
		return ErrEmptyInput
	}
	for _, x := range xs {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return ErrInvalidValue
		}
	}
	return nil
}

// Median calculates the sample median without mutating the input slice.
func Median(xs []float64) (float64, error) {
	if err := validateSlice(xs); err != nil {
		return 0, err
	}
	n := len(xs)
	if n == 1 {
		return xs[0], nil
	}

	copied := make([]float64, n)
	copy(copied, xs)
	sort.Float64s(copied)

	if n%2 == 1 {
		return copied[n/2], nil
	}
	return (copied[n/2-1] + copied[n/2]) / 2.0, nil
}

// Mean calculates the arithmetic mean of the slice.
func Mean(xs []float64) (float64, error) {
	if err := validateSlice(xs); err != nil {
		return 0, err
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs)), nil
}

// StdDev calculates the sample standard deviation with Bessel's correction (N-1).
// For a single sample, StdDev returns 0.
func StdDev(xs []float64) (float64, error) {
	if err := validateSlice(xs); err != nil {
		return 0, err
	}
	n := len(xs)
	if n <= 1 {
		return 0, nil
	}

	mean, _ := Mean(xs)
	var sumSq float64
	for _, x := range xs {
		diff := x - mean
		sumSq += diff * diff
	}
	variance := sumSq / float64(n-1)
	return math.Sqrt(variance), nil
}

// CV calculates the coefficient of variation as a percentage: (stddev / mean) * 100.
func CV(xs []float64) (float64, error) {
	if err := validateSlice(xs); err != nil {
		return 0, err
	}
	if len(xs) <= 1 {
		return 0, nil
	}

	mean, _ := Mean(xs)
	stddev, _ := StdDev(xs)

	if mean == 0 {
		if stddev == 0 {
			return 0, nil
		}
		return 0, ErrZeroMean
	}

	return (stddev / mean) * 100.0, nil
}

// DeltaPct calculates the percentage change from base to head: ((head - base) / base) * 100.
func DeltaPct(base, head float64) (float64, error) {
	if math.IsNaN(base) || math.IsNaN(head) || math.IsInf(base, 0) || math.IsInf(head, 0) {
		return 0, ErrInvalidValue
	}
	if base == 0 {
		if head == 0 {
			return 0, nil
		}
		return 0, ErrZeroBase
	}
	return ((head - base) / base) * 100.0, nil
}

// MannWhitneyU performs a two-sided Mann-Whitney U test between base and head samples.
// It handles ties via average ranking and returns the U statistic and p-value.
func MannWhitneyU(base, head []float64) (u float64, p float64, err error) {
	if err := validateSlice(base); err != nil {
		return 0, 0, fmt.Errorf("base: %w", err)
	}
	if err := validateSlice(head); err != nil {
		return 0, 0, fmt.Errorf("head: %w", err)
	}

	// Check if all elements in both base and head are identical
	first := base[0]
	allSame := true
	for _, v := range base {
		if v != first {
			allSame = false
			break
		}
	}
	if allSame {
		for _, v := range head {
			if v != first {
				allSame = false
				break
			}
		}
	}
	if allSame {
		// Identical data produces p = 1 and mid-rank U
		return float64(len(base)*len(head)) / 2.0, 1.0, nil
	}

	res, err := moremath.MannWhitneyUTest(base, head, moremath.LocationDiffers)
	if err != nil {
		if errors.Is(err, moremath.ErrSamplesEqual) {
			return float64(len(base)*len(head)) / 2.0, 1.0, nil
		}
		return 0, 0, fmt.Errorf("mann-whitney: %w", err)
	}

	return res.U, res.P, nil
}

// BootstrapCI computes a percentile bootstrap confidence interval for the percentage
// difference in medians between base and head.
// The function is purely deterministic for a given seed.
func BootstrapCI(base, head []float64, confidence float64, iters int, seed int64) ([2]float64, error) {
	if err := validateSlice(base); err != nil {
		return [2]float64{}, fmt.Errorf("base: %w", err)
	}
	if err := validateSlice(head); err != nil {
		return [2]float64{}, fmt.Errorf("head: %w", err)
	}
	if confidence <= 0 || confidence >= 1 {
		return [2]float64{}, ErrInvalidConfidence
	}
	if iters <= 0 {
		iters = 1000
	}

	n1 := len(base)
	n2 := len(head)

	// #nosec G404 -- pseudo-random numbers with explicit seed are required for deterministic testing
	rng := rand.New(rand.NewSource(seed))

	bSample := make([]float64, n1)
	hSample := make([]float64, n2)
	deltas := make([]float64, 0, iters)

	for i := 0; i < iters; i++ {
		for j := 0; j < n1; j++ {
			bSample[j] = base[rng.Intn(n1)]
		}
		for j := 0; j < n2; j++ {
			hSample[j] = head[rng.Intn(n2)]
		}

		bMed, _ := Median(bSample)
		hMed, _ := Median(hSample)

		d, err := DeltaPct(bMed, hMed)
		if err != nil {
			if errors.Is(err, ErrZeroBase) {
				return [2]float64{}, ErrZeroBase
			}
			return [2]float64{}, err
		}
		deltas = append(deltas, d)
	}

	sort.Float64s(deltas)

	alpha := (1.0 - confidence) / 2.0
	loIdx := int(math.Floor(alpha * float64(iters)))
	if loIdx < 0 {
		loIdx = 0
	}
	hiIdx := int(math.Ceil((1.0-alpha)*float64(iters))) - 1
	if hiIdx >= len(deltas) {
		hiIdx = len(deltas) - 1
	}

	return [2]float64{deltas[loIdx], deltas[hiIdx]}, nil
}

// GeoMean calculates the geometric mean of positive values.
func GeoMean(xs []float64) (float64, error) {
	if err := validateSlice(xs); err != nil {
		return 0, err
	}
	var sumLog float64
	for _, x := range xs {
		if x <= 0 {
			return 0, fmt.Errorf("stats: non-positive value %g for geometric mean", x)
		}
		sumLog += math.Log(x)
	}
	return math.Exp(sumLog / float64(len(xs))), nil
}

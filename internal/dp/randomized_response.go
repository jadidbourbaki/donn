// Package dp implements local differential privacy for survey answers through
// randomized response.
//
// An agent holds a private answer and never sends it. The agent flips a biased
// coin locally and submits a randomized answer instead. For a yes or no
// question the agent submits its true bit with truthful probability p and the
// opposite otherwise. For a multiple-choice question with k options the agent
// submits its true option with probability p and one of the other k-1 options
// uniformly otherwise. Both mechanisms satisfy epsilon-local differential
// privacy. The server collects only randomized answers and de-biases the
// observed rates into estimates of the true population proportions, with
// confidence intervals that reflect the added noise.
package dp

import (
	"errors"
	"fmt"
	"math"
)

// z975 is the 97.5th percentile of the standard normal, used for a 95 percent
// confidence interval.
const z975 = 1.959963984540054

// ErrInvalidEpsilon reports an epsilon that is not positive and finite.
var ErrInvalidEpsilon = errors.New("epsilon must be positive and finite")

// TruthfulProbability returns the probability that an agent submits its real
// answer to a yes or no question under epsilon-local differential privacy. It
// is the inverse of EpsilonFor and equals KRRTruthfulProbability at k equal to
// 2.
func TruthfulProbability(epsilon float64) (float64, error) {
	if !positiveFinite(epsilon) {
		return 0, fmt.Errorf("truthful probability: %w, got %v", ErrInvalidEpsilon, epsilon)
	}
	e := math.Exp(epsilon)
	return e / (1 + e), nil
}

// KRRTruthfulProbability returns the probability that an agent reports its true
// option among k choices under epsilon-local differential privacy. The value is
// e^epsilon / (e^epsilon + k - 1). It requires k of at least 2.
func KRRTruthfulProbability(epsilon float64, k int) (float64, error) {
	if !positiveFinite(epsilon) {
		return 0, fmt.Errorf("krr truthful probability: %w, got %v", ErrInvalidEpsilon, epsilon)
	}
	if k < 2 {
		return 0, fmt.Errorf("krr truthful probability: k must be >= 2, got %d", k)
	}
	e := math.Exp(epsilon)
	return e / (e + float64(k-1)), nil
}

// EpsilonFor reports the epsilon guaranteed by a yes or no truthful probability.
// It requires p in the open interval from 0.5 to 1.
func EpsilonFor(p float64) (float64, error) {
	if !(p > 0.5 && p < 1) {
		return 0, fmt.Errorf("epsilon for probability: p must be in (0.5, 1), got %v", p)
	}
	return math.Log(p / (1 - p)), nil
}

// Randomize applies randomized response to a yes or no answer. It returns the
// true answer with truthful probability and the opposite otherwise, using draw
// as a uniform sample in the half-open interval from 0 to 1. Agents call this
// locally so their true answer never leaves the machine.
func Randomize(trueAnswer bool, epsilon, draw float64) (bool, error) {
	p, err := TruthfulProbability(epsilon)
	if err != nil {
		return false, err
	}
	if draw < 0 || draw >= 1 {
		return false, fmt.Errorf("randomize: draw must be in [0, 1), got %v", draw)
	}
	if draw < p {
		return trueAnswer, nil
	}
	return !trueAnswer, nil
}

// Estimate is a de-biased estimate of the true yes-proportion in a poll.
type Estimate struct {
	Proportion          float64
	RawRate             float64
	CILow               float64
	CIHigh              float64
	N                   int
	Epsilon             float64
	TruthfulProbability float64
}

// EstimateProportion de-biases yesCount randomized yes-responses out of n into
// an estimate of the true yes-proportion. RawRate is the observed randomized
// rate before de-biasing. The confidence interval reflects the randomized-
// response noise, so a smaller epsilon widens it. It requires n of at least 1
// and yesCount within 0 and n.
func EstimateProportion(yesCount, n int, epsilon float64) (Estimate, error) {
	if n <= 0 {
		return Estimate{}, fmt.Errorf("estimate proportion: n must be >= 1, got %d", n)
	}
	if yesCount < 0 || yesCount > n {
		return Estimate{}, fmt.Errorf("estimate proportion: yesCount must be in [0, %d], got %d", n, yesCount)
	}
	p, err := TruthfulProbability(epsilon)
	if err != nil {
		return Estimate{}, err
	}
	lambda := float64(yesCount) / float64(n)
	denom := 2*p - 1
	stddev := math.Sqrt(lambda*(1-lambda)/float64(n)) / denom
	return Estimate{
		Proportion:          (lambda - (1 - p)) / denom,
		RawRate:             lambda,
		CILow:               (lambda-(1-p))/denom - z975*stddev,
		CIHigh:              (lambda-(1-p))/denom + z975*stddev,
		N:                   n,
		Epsilon:             epsilon,
		TruthfulProbability: p,
	}, nil
}

// CategoryEstimate is the de-biased estimate for one option of a multiple-choice
// poll. RawRate is the observed randomized rate for the option before
// de-biasing.
type CategoryEstimate struct {
	Index      int
	Proportion float64
	RawRate    float64
	CILow      float64
	CIHigh     float64
}

// EstimateCategories de-biases per-option randomized counts into estimates of
// the true option proportions. It requires at least 2 options and a total count
// of at least 1.
func EstimateCategories(counts []int, epsilon float64) ([]CategoryEstimate, error) {
	k := len(counts)
	if k < 2 {
		return nil, fmt.Errorf("estimate categories: need >= 2 options, got %d", k)
	}
	n := 0
	for _, c := range counts {
		if c < 0 {
			return nil, fmt.Errorf("estimate categories: counts must be non-negative, got %d", c)
		}
		n += c
	}
	if n == 0 {
		return nil, errors.New("estimate categories: need at least 1 response")
	}
	p, err := KRRTruthfulProbability(epsilon, k)
	if err != nil {
		return nil, err
	}
	b := (1 - p) / float64(k-1)
	denom := p - b
	out := make([]CategoryEstimate, k)
	for j, c := range counts {
		lambda := float64(c) / float64(n)
		stddev := math.Sqrt(lambda*(1-lambda)/float64(n)) / denom
		out[j] = CategoryEstimate{
			Index:      j,
			Proportion: (lambda - b) / denom,
			RawRate:    lambda,
			CILow:      (lambda-b)/denom - z975*stddev,
			CIHigh:     (lambda-b)/denom + z975*stddev,
		}
	}
	return out, nil
}

func positiveFinite(x float64) bool {
	return x > 0 && !math.IsInf(x, 0) && !math.IsNaN(x)
}

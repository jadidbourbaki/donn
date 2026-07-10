// Package dp implements local differential privacy for binary survey answers
// through randomized response.
//
// An agent holds a private yes or no answer and never sends it. The agent
// flips a biased coin and submits a randomized bit instead. With truthful
// probability p the agent submits its real answer, and with probability 1-p it
// submits the opposite. The mechanism satisfies epsilon-local differential
// privacy where epsilon equals ln(p/(1-p)), so no observer, including the
// survey server, can recover any single agent's answer beyond that bound. The
// server collects only randomized bits and de-biases the observed yes-rate into
// an estimate of the true population proportion.
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
// answer to achieve epsilon-local differential privacy. It is the inverse of
// EpsilonFor.
func TruthfulProbability(epsilon float64) (float64, error) {
	if epsilon <= 0 || math.IsInf(epsilon, 0) || math.IsNaN(epsilon) {
		return 0, fmt.Errorf("truthful probability: %w, got %v", ErrInvalidEpsilon, epsilon)
	}
	e := math.Exp(epsilon)
	return e / (1 + e), nil
}

// EpsilonFor reports the epsilon guaranteed by a truthful probability. It
// requires p in the open interval from 0.5 to 1.
func EpsilonFor(p float64) (float64, error) {
	if !(p > 0.5 && p < 1) {
		return 0, fmt.Errorf("epsilon for probability: p must be in (0.5, 1), got %v", p)
	}
	return math.Log(p / (1 - p)), nil
}

// Randomize applies randomized response to a true answer. It returns the true
// answer with truthful probability and the opposite otherwise, using draw as a
// uniform sample in the half-open interval from 0 to 1. Agents call this
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
	CILow               float64
	CIHigh              float64
	N                   int
	Epsilon             float64
	TruthfulProbability float64
}

// EstimateProportion de-biases yesCount randomized yes-responses out of n into
// an estimate of the true yes-proportion. The confidence interval reflects the
// randomized-response noise, so a smaller epsilon widens it. It requires n to
// be at least 1 and yesCount within 0 and n.
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
	proportion := (lambda - (1 - p)) / denom
	stddev := math.Sqrt(lambda*(1-lambda)/float64(n)) / denom
	return Estimate{
		Proportion:          proportion,
		CILow:               proportion - z975*stddev,
		CIHigh:              proportion + z975*stddev,
		N:                   n,
		Epsilon:             epsilon,
		TruthfulProbability: p,
	}, nil
}

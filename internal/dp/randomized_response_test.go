package dp

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruthfulProbability_CalibratesAndInverts(t *testing.T) {
	tests := []struct {
		name    string
		epsilon float64
	}{
		{name: "eps 0.5", epsilon: 0.5},
		{name: "eps 1", epsilon: 1.0},
		{name: "eps 3", epsilon: 3.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := TruthfulProbability(tt.epsilon)
			require.NoError(t, err)
			assert.Greater(t, p, 0.5)
			assert.Less(t, p, 1.0)
			back, err := EpsilonFor(p)
			require.NoError(t, err)
			assert.InDelta(t, tt.epsilon, back, 1e-9)
		})
	}
}

func TestTruthfulProbability_SmallerEpsilonIsNoisier(t *testing.T) {
	tight, err := TruthfulProbability(0.5)
	require.NoError(t, err)
	loose, err := TruthfulProbability(3.0)
	require.NoError(t, err)
	assert.Less(t, tight, loose)
}

func TestTruthfulProbability_RejectsBadEpsilon(t *testing.T) {
	for _, bad := range []float64{0, -1, math.Inf(1), math.NaN()} {
		_, err := TruthfulProbability(bad)
		assert.ErrorIs(t, err, ErrInvalidEpsilon)
	}
}

func TestRandomize_FollowsTheDraw(t *testing.T) {
	// p for eps=1 is about 0.731. A draw below p reports the truth, above flips.
	truthful, err := Randomize(true, 1.0, 0.0)
	require.NoError(t, err)
	assert.True(t, truthful)

	flipped, err := Randomize(true, 1.0, 0.99)
	require.NoError(t, err)
	assert.False(t, flipped)
}

func TestRandomize_RejectsOutOfRangeDraw(t *testing.T) {
	_, err := Randomize(true, 1.0, 1.0)
	assert.Error(t, err)
	_, err = Randomize(true, 1.0, -0.1)
	assert.Error(t, err)
}

func TestEstimateProportion_RecoversTruthWithoutNoiseAtDraw(t *testing.T) {
	// With no flips (all agents truthful), the de-biased estimate equals the
	// raw yes-rate. Build a tally that is exactly 70 percent yes.
	est, err := EstimateProportion(70, 100, 1.0)
	require.NoError(t, err)
	// Feed the debias the raw rate a fully-truthful population would produce.
	// Here we check the estimator maps a known lambda back through the formula.
	p, err := TruthfulProbability(1.0)
	require.NoError(t, err)
	lambda := 70.0 / 100.0
	want := (lambda - (1 - p)) / (2*p - 1)
	assert.InDelta(t, want, est.Proportion, 1e-9)
	assert.Less(t, est.CILow, est.Proportion)
	assert.Greater(t, est.CIHigh, est.Proportion)
}

func TestEstimateProportion_SmallerEpsilonWidensInterval(t *testing.T) {
	tight, err := EstimateProportion(60, 100, 3.0)
	require.NoError(t, err)
	loose, err := EstimateProportion(60, 100, 0.5)
	require.NoError(t, err)
	assert.Less(t, tight.CIHigh-tight.CILow, loose.CIHigh-loose.CILow)
}

func TestEstimateProportion_RejectsBadCounts(t *testing.T) {
	_, err := EstimateProportion(0, 0, 1.0)
	assert.Error(t, err)
	_, err = EstimateProportion(5, 3, 1.0)
	assert.Error(t, err)
}

func TestKRRTruthfulProbability_MatchesBinaryAtK2(t *testing.T) {
	binary, err := TruthfulProbability(1.0)
	require.NoError(t, err)
	krr, err := KRRTruthfulProbability(1.0, 2)
	require.NoError(t, err)
	assert.InDelta(t, binary, krr, 1e-12)
}

func TestKRRTruthfulProbability_RejectsBadArgs(t *testing.T) {
	_, err := KRRTruthfulProbability(1.0, 1)
	assert.Error(t, err)
	_, err = KRRTruthfulProbability(0, 3)
	assert.Error(t, err)
}

func TestEstimateCategories_ProportionsAreDebiasedAndSumNearOne(t *testing.T) {
	est, err := EstimateCategories([]int{50, 30, 20}, 1.0)
	require.NoError(t, err)
	require.Len(t, est, 3)

	var sum float64
	for _, c := range est {
		sum += c.Proportion
		assert.Less(t, c.CILow, c.CIHigh)
	}
	// De-biased proportions of a full partition sum to 1.
	assert.InDelta(t, 1.0, sum, 1e-9)
	// The most-reported option keeps the largest de-biased proportion.
	assert.Greater(t, est[0].Proportion, est[1].Proportion)
	assert.Greater(t, est[1].Proportion, est[2].Proportion)
}

func TestEstimateCategories_RejectsBadInput(t *testing.T) {
	_, err := EstimateCategories([]int{5}, 1.0)
	assert.Error(t, err)
	_, err = EstimateCategories([]int{0, 0, 0}, 1.0)
	assert.Error(t, err)
}

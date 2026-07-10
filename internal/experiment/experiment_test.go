package experiment

import (
	"context"
	"math"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jadidbourbaki/donn/internal/api"
	"github.com/jadidbourbaki/donn/internal/survey"
)

func TestRun_DebiasingRecoversTheTruth(t *testing.T) {
	ts := httptest.NewServer(api.NewServer(survey.NewStore()))
	t.Cleanup(ts.Close)

	const trueProp = 0.7
	results, err := Run(context.Background(), ts.Client(), ts.URL, Config{
		Question:    "test poll",
		TrueProp:    trueProp,
		Epsilon:     1.0,
		Seed:        42,
		Checkpoints: []int{4000},
		Concurrency: 32,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)

	got := results[0]
	// The de-biased estimate lands near the realized true proportion.
	assert.InDelta(t, got.TrueProp, got.Debiased, 0.05)
	// And it is closer to the truth than the naive read of the noised responses,
	// which is biased toward one half.
	assert.Less(t, math.Abs(got.Debiased-got.TrueProp), math.Abs(got.RawRate-got.TrueProp))
	assert.Less(t, got.RawRate, trueProp)
}

func TestRun_RequiresCheckpoints(t *testing.T) {
	ts := httptest.NewServer(api.NewServer(survey.NewStore()))
	t.Cleanup(ts.Close)

	_, err := Run(context.Background(), ts.Client(), ts.URL, Config{
		TrueProp: 0.5,
		Epsilon:  1.0,
	})
	assert.Error(t, err)
}

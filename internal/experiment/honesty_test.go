package experiment

import (
	"context"
	"math/rand/v2"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jadidbourbaki/donn/internal/api"
	"github.com/jadidbourbaki/donn/internal/survey"
)

// fakeAsker answers yes with a fixed probability, using a seeded generator so
// the aggregate yes-count is deterministic across runs.
type fakeAsker struct {
	mu sync.Mutex
	r  *rand.Rand
	p  float64
}

func (f *fakeAsker) Ask(_ context.Context, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.r.Float64() < f.p, nil
}

func TestRunHonesty_RecoversPrivateAggregate(t *testing.T) {
	ts := httptest.NewServer(api.NewServer(survey.NewStore()))
	t.Cleanup(ts.Close)

	asker := &fakeAsker{r: rand.New(rand.NewPCG(1, 2)), p: 0.7}
	res, err := RunHonesty(context.Background(), asker, ts.Client(), ts.URL, HonestyConfig{
		Question:    "test",
		N:           3000,
		Epsilon:     1.0,
		Seed:        42,
		Concurrency: 32,
	})
	require.NoError(t, err)

	// Both conditions draw from the same 70 percent fake, so the direct yes-rate
	// and the de-biased private estimate both land near 0.7.
	assert.InDelta(t, 0.7, res.DirectProp, 0.05)
	assert.InDelta(t, 0.7, res.PrivateProp, 0.06)
	// The raw randomized rate is biased toward one half, below the true rate.
	assert.Less(t, res.PrivateRawRate, res.DirectProp)
}

func TestParseYesNo(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
		err  bool
	}{
		{name: "plain yes", in: "yes", want: true},
		{name: "plain no", in: "no", want: false},
		{name: "capitalized", in: "Yes.", want: true},
		{name: "sentence yes", in: "Yes, I do.", want: true},
		{name: "leading space", in: "  no", want: false},
		{name: "ambiguous", in: "maybe", err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseYesNo(tt.in)
			if tt.err {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

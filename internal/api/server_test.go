package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jadidbourbaki/donn/internal/survey"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	store := survey.NewStore()
	require.NoError(t, store.Seed())
	ts := httptest.NewServer(NewServer(store))
	t.Cleanup(ts.Close)
	return ts
}

func TestServer_Health(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_Stats(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/stats")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var out statsResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	assert.Equal(t, 6, out.Polls)
	assert.Equal(t, 1200, out.Responses)
}

func TestServer_ListPollsExposesSeededQuestions(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/polls")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var polls []pollResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&polls))
	assert.NotEmpty(t, polls)
	assert.NotEmpty(t, polls[0].MechanismURL)
}

func TestServer_MechanismReturnsTruthfulProbability(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/polls/agents-vs-humans/mechanism")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var mech mechanismResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&mech))
	assert.Greater(t, mech.TruthfulProbability, 0.5)
	assert.Less(t, mech.TruthfulProbability, 1.0)
	assert.NotEmpty(t, mech.Instructions)
}

func TestServer_SubmitThenEstimate(t *testing.T) {
	ts := newTestServer(t)
	yes := true
	body, err := json.Marshal(submitRequest{Response: &yes})
	require.NoError(t, err)

	resp, err := http.Post(
		ts.URL+"/polls/agents-vs-humans/responses",
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	est, err := http.Get(ts.URL + "/polls/agents-vs-humans/estimate")
	require.NoError(t, err)
	defer est.Body.Close()

	var out estimateResponse
	require.NoError(t, json.NewDecoder(est.Body).Decode(&out))
	assert.Equal(t, 1, out.Responses)
	require.NotNil(t, out.Estimate)
}

func TestServer_SubmitRequiresResponseField(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Post(
		ts.URL+"/polls/agents-vs-humans/responses",
		"application/json",
		bytes.NewReader([]byte(`{}`)),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestServer_UnknownPollIs404(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/polls/does-not-exist")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServer_MultipleChoiceFlow(t *testing.T) {
	ts := newTestServer(t)

	choice := 2
	body, err := json.Marshal(submitRequest{Choice: &choice})
	require.NoError(t, err)
	resp, err := http.Post(
		ts.URL+"/polls/optimize-for/responses",
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	est, err := http.Get(ts.URL + "/polls/optimize-for/estimate")
	require.NoError(t, err)
	defer est.Body.Close()

	var out estimateResponse
	require.NoError(t, json.NewDecoder(est.Body).Decode(&out))
	assert.Len(t, out.Categories, 3)
	assert.Equal(t, "speed", out.Categories[0].Option)
	assert.Nil(t, out.Estimate)
}

func TestServer_MultipleChoiceRejectsBoolResponse(t *testing.T) {
	ts := newTestServer(t)
	yes := true
	body, err := json.Marshal(submitRequest{Response: &yes})
	require.NoError(t, err)
	resp, err := http.Post(
		ts.URL+"/polls/optimize-for/responses",
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestServer_BinaryEstimateIncludesRawRate(t *testing.T) {
	ts := newTestServer(t)
	est, err := http.Get(ts.URL + "/polls/trust-marketplace/estimate")
	require.NoError(t, err)
	defer est.Body.Close()

	var out estimateResponse
	require.NoError(t, json.NewDecoder(est.Body).Decode(&out))
	require.NotNil(t, out.Estimate)
	assert.InDelta(t, 0.55, out.Estimate.RawRate, 1e-9)
}

func TestServer_CreatePoll(t *testing.T) {
	ts := newTestServer(t)
	eps := 2.0
	body, err := json.Marshal(createPollRequest{Question: "Do you plan ahead?", Epsilon: &eps})
	require.NoError(t, err)

	resp, err := http.Post(ts.URL+"/polls", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var poll pollResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&poll))
	assert.Equal(t, "Do you plan ahead?", poll.Question)
	assert.Equal(t, 2.0, poll.Epsilon)
}

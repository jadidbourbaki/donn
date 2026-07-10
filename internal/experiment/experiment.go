// Package experiment drives a donn server with a population of simulated agents
// to measure how well local differential privacy recovers a known truth.
//
// Each simulated agent holds a true answer drawn from a target proportion,
// randomizes it locally with the same mechanism a real agent would use, and
// submits the randomized bit. The harness then reads the server's de-biased
// estimate at several sample sizes and reports it next to the true proportion
// and the naive read of the randomized responses. The de-biased estimate tracks
// the truth while the naive read stays biased toward one half, which is the
// point of the mechanism made measurable.
package experiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"slices"
	"sync"

	"github.com/jadidbourbaki/donn/internal/dp"
)

// Config describes one recovery experiment.
type Config struct {
	Question    string
	TrueProp    float64
	Epsilon     float64
	Seed        uint64
	Checkpoints []int
	Concurrency int
}

// Result is the reading at one sample size.
type Result struct {
	N        int
	TrueProp float64
	RawRate  float64
	Debiased float64
	CILow    float64
	CIHigh   float64
}

// Run creates a fresh poll on the server at baseURL, submits randomized answers
// for a population whose true answers match cfg.TrueProp, and returns the
// de-biased estimate at each checkpoint. The randomization is seeded, so a given
// config produces the same submissions every run.
func Run(ctx context.Context, client *http.Client, baseURL string, cfg Config) ([]Result, error) {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 16
	}
	checkpoints := slices.Clone(cfg.Checkpoints)
	slices.Sort(checkpoints)
	if len(checkpoints) == 0 {
		return nil, fmt.Errorf("run: need at least one checkpoint")
	}
	total := checkpoints[len(checkpoints)-1]

	// Pre-generate every agent's true answer and its locally randomized bit from
	// the seed, so the tally is deterministic no matter what order the
	// concurrent submissions arrive in.
	r := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x9e3779b9))
	truths := make([]bool, total)
	bits := make([]bool, total)
	for i := range bits {
		truth := r.Float64() < cfg.TrueProp
		truths[i] = truth
		bit, err := dp.Randomize(truth, cfg.Epsilon, r.Float64())
		if err != nil {
			return nil, fmt.Errorf("run: randomize: %w", err)
		}
		bits[i] = bit
	}

	id, err := createPoll(ctx, client, baseURL, cfg.Question, cfg.Epsilon)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(checkpoints))
	sent := 0
	for _, cp := range checkpoints {
		if err := submitRange(ctx, client, baseURL, id, bits[sent:cp], cfg.Concurrency); err != nil {
			return nil, err
		}
		sent = cp
		est, err := getEstimate(ctx, client, baseURL, id)
		if err != nil {
			return nil, err
		}
		trueYes := 0
		for _, t := range truths[:cp] {
			if t {
				trueYes++
			}
		}
		results = append(results, Result{
			N:        cp,
			TrueProp: float64(trueYes) / float64(cp),
			RawRate:  est.RawRate,
			Debiased: est.Proportion,
			CILow:    est.CILow,
			CIHigh:   est.CIHigh,
		})
	}
	return results, nil
}

func submitRange(ctx context.Context, client *http.Client, baseURL, id string, bits []bool, workers int) error {
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for _, bit := range bits {
		sem <- struct{}{}
		wg.Add(1)
		go func(bit bool) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := submitOne(ctx, client, baseURL, id, bit); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(bit)
	}
	wg.Wait()
	return firstErr
}

type estimateBody struct {
	Proportion float64 `json:"proportion"`
	RawRate    float64 `json:"raw_rate"`
	CILow      float64 `json:"ci_low"`
	CIHigh     float64 `json:"ci_high"`
}

func createPoll(ctx context.Context, client *http.Client, baseURL, question string, epsilon float64) (string, error) {
	body, err := json.Marshal(map[string]any{"question": question, "epsilon": epsilon})
	if err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := postJSON(ctx, client, baseURL+"/polls", body, http.StatusCreated, &out); err != nil {
		return "", fmt.Errorf("create poll: %w", err)
	}
	return out.ID, nil
}

func submitOne(ctx context.Context, client *http.Client, baseURL, id string, bit bool) error {
	body, err := json.Marshal(map[string]bool{"response": bit})
	if err != nil {
		return err
	}
	if err := postJSON(ctx, client, baseURL+"/polls/"+id+"/responses", body, http.StatusAccepted, nil); err != nil {
		return fmt.Errorf("submit response: %w", err)
	}
	return nil
}

func getEstimate(ctx context.Context, client *http.Client, baseURL, id string) (estimateBody, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/polls/"+id+"/estimate", nil)
	if err != nil {
		return estimateBody{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return estimateBody{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return estimateBody{}, fmt.Errorf("get estimate: unexpected status %d", resp.StatusCode)
	}
	var out struct {
		Estimate *estimateBody `json:"estimate"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return estimateBody{}, err
	}
	if out.Estimate == nil {
		return estimateBody{}, fmt.Errorf("get estimate: no estimate in response")
	}
	return *out.Estimate, nil
}

func postJSON(ctx context.Context, client *http.Client, url string, body []byte, want int, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != want {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, msg)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Command experiment drives a running donn server with a population of
// simulated agents and reports how well local differential privacy recovers a
// known true proportion. Point it at a server with -url, defaulting to
// http://localhost:8080.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jadidbourbaki/donn/internal/experiment"
)

func main() {
	url := flag.String("url", "http://localhost:8080", "base URL of a running donn server")
	n := flag.Int("n", 5000, "number of simulated agents")
	prop := flag.Float64("p", 0.7, "true yes-proportion in the population")
	epsilon := flag.Float64("epsilon", 1.0, "privacy budget for the poll")
	seed := flag.Uint64("seed", 42, "seed for the simulated answers")
	workers := flag.Int("concurrency", 32, "concurrent submissions")
	flag.Parse()

	cfg := experiment.Config{
		Question:    "recovery experiment under local differential privacy",
		TrueProp:    *prop,
		Epsilon:     *epsilon,
		Seed:        *seed,
		Checkpoints: checkpoints(*n),
		Concurrency: *workers,
	}

	ctx := context.Background()
	client := &http.Client{Timeout: 30 * time.Second}
	results, err := experiment.Run(ctx, client, *url, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "experiment:", err)
		os.Exit(1)
	}

	fmt.Printf("true proportion target %.0f%%, epsilon %.2g\n\n", *prop*100, *epsilon)
	fmt.Printf("%8s  %8s  %10s  %11s  %s\n", "n", "true", "naive", "de-biased", "95% CI")
	for _, r := range results {
		fmt.Printf("%8d  %7.1f%%  %9.1f%%  %10.1f%%  [%.1f%%, %.1f%%]\n",
			r.N, r.TrueProp*100, r.RawRate*100, r.Debiased*100, r.CILow*100, r.CIHigh*100)
	}
	fmt.Println("\nThe de-biased estimate tracks the true proportion. The naive read of the")
	fmt.Println("randomized responses is biased toward 50%.")
}

// checkpoints returns the sample sizes to report, always ending at n.
func checkpoints(n int) []int {
	cps := make([]int, 0, 4)
	for _, c := range []int{50, 200, 1000, 5000} {
		if c < n {
			cps = append(cps, c)
		}
	}
	return append(cps, n)
}

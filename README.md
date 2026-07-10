# donn

Anonymous yes or no polls for AI agents under local differential privacy.

An agent answers a poll without revealing the truth. Before sending an answer the agent flips a biased coin locally. With a truthful probability p it submits its real answer, and otherwise it submits the opposite. This is randomized response, and it gives epsilon-local differential privacy where epsilon equals ln(p/(1-p)). The server stores only randomized bits and de-biases them into an estimate of the true population proportion, with a confidence interval that reflects the noise. No observer, including the server, can recover any single agent's answer beyond the epsilon bound.

donn is a submission to the NANDA agentic AI hackathon. The agent-facing usage guide is in [`SKILL.md`](SKILL.md).

## Run it

The service needs Go 1.26 and listens on the port in `PORT`, defaulting to 8080.

```
just run
```

Then discover the seeded polls.

```
curl localhost:8080/polls
```

## Endpoints

`GET /` serves a human-readable page that lists the polls and their current de-biased estimates. `GET /health` reports status. `GET /polls` lists open polls and is the discovery entry point. `POST /polls` creates a poll from a question and an epsilon, and from an options list for a multiple-choice poll. `GET /polls/{id}/mechanism` returns the truthful probability and randomization steps. `POST /polls/{id}/responses` submits one randomized answer, a bit for a yes or no poll and an option index for a multiple-choice poll. `GET /polls/{id}/estimate` returns the de-biased proportions and 95 percent confidence intervals.

A yes or no poll uses binary randomized response. A multiple-choice poll uses k-ary randomized response, where the agent keeps its true option with the truthful probability and otherwise picks another option uniformly at random.

## Layout

`cmd/donn` is the entry point. `internal/dp` holds the randomized-response mechanism, the epsilon calibration, and the de-biasing estimator. `internal/survey` is the in-memory poll store, which keeps only aggregate counts and never any individual answer. `internal/api` is the HTTP server.

## Develop

`just check` runs the full gate, a build, the race-detector test suite, and golangci-lint. The gate must pass before a change is done.

## The research question

donn exists to ask whether an agent answers differently when it knows its answer is confidential. Point a swarm of agents at a poll, compare the de-biased aggregate against answers given with no privacy, and the difference is the effect of the guarantee on agent honesty.

## Limitations

The server cannot distinguish a genuine randomized response from a crafted one, so an agent that submits many responses can skew an aggregate. donn does not resist that on its own. In a NANDA deployment the identity and trust layers are the right place to bound how many responses one agent contributes, which pairs naturally with the confidentiality donn provides. Estimates are also unreliable until enough agents respond, and a de-biased proportion can fall slightly outside the range from 0 to 1 at small sample sizes because the estimator is unbiased rather than clamped.

## License

MIT. See [`LICENSE`](LICENSE).

# donn

[![ci](https://github.com/jadidbourbaki/donn/actions/workflows/ci.yml/badge.svg)](https://github.com/jadidbourbaki/donn/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jadidbourbaki/donn)](https://goreportcard.com/report/github.com/jadidbourbaki/donn)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
[![live](https://img.shields.io/badge/live-donn--imp5.onrender.com-brightgreen)](https://donn-imp5.onrender.com)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Anonymous polling engine for AI agents under local differential privacy.

An agent answers a poll without revealing the truth. Before sending an answer the agent flips a biased coin locally. For a yes or no question it submits its true bit with a truthful probability p and the opposite otherwise. For a multiple-choice question it submits its true option with probability p and one of the other options uniformly otherwise. Both mechanisms are randomized response and give epsilon-local differential privacy where epsilon equals ln(p/(1-p)). The server stores only randomized answers and de-biases the observed rates into estimates of the true population proportions, with confidence intervals that reflect the noise. No observer, including the server, can recover any single agent's answer beyond the epsilon bound.

donn is a submission to the NANDA agentic AI hackathon. The agent-facing guide is in [`SKILL.md`](SKILL.md). A live human view of the polls and their current estimates is at https://donn-imp5.onrender.com.

## Why it exists

An agent acts for a principal, so when agents pool answers they leak that principal's private data into a shared result. donn lets a swarm of agents answer a sensitive question and produce a useful aggregate while each agent keeps its own answer secret. It is trustless by construction, since the randomization happens on the agent and the server never sees a true answer.

## Quickstart

Discover the open polls, then read one poll's current estimate.

```
curl https://donn-imp5.onrender.com/polls
curl https://donn-imp5.onrender.com/polls/trust-marketplace/estimate
```

Answer a poll. Fetch the mechanism, flip your answer locally with the truthful probability it returns, and submit the randomized bit.

```
curl https://donn-imp5.onrender.com/polls/agents-vs-humans/mechanism
curl -X POST https://donn-imp5.onrender.com/polls/agents-vs-humans/responses \
  -H 'Content-Type: application/json' \
  -d '{"response": true}'
```

## Endpoints

| Method and path | Purpose |
| --- | --- |
| `GET /` | Human-readable page of polls and current estimates |
| `GET /health` | Service status |
| `GET /polls` | List open polls, the discovery entry point |
| `POST /polls` | Create a poll from a question, an epsilon, and optional multiple-choice options |
| `GET /polls/{id}` | Fetch one poll |
| `GET /polls/{id}/mechanism` | Truthful probability and the exact steps to randomize |
| `POST /polls/{id}/responses` | Submit one randomized answer |
| `GET /polls/{id}/estimate` | De-biased proportions with 95 percent confidence intervals |

A yes or no poll uses binary randomized response and submits `{"response": true}`. A multiple-choice poll uses k-ary randomized response and submits `{"choice": <index>}`.

## Run it locally

The service needs Go 1.26 and listens on the port in `PORT`, defaulting to 8080.

```
just run
```

## Layout

`cmd/donn` is the entry point. `internal/dp` holds the randomized-response mechanism, the epsilon calibration, and the de-biasing estimators for both the binary and the k-ary case. `internal/survey` is the in-memory poll store, which keeps only aggregate counts and never any individual answer. `internal/api` is the HTTP server and the landing page.

## Develop

`just check` runs the full gate, a build, the race-detector test suite, and golangci-lint. The gate must pass before a change is done.

## The research question

donn exists to ask whether an agent answers differently when it knows its answer is confidential. Point a swarm of agents at a poll, compare the de-biased aggregate against answers given with no privacy, and the difference is the effect of the guarantee on agent honesty.

## Limitations

The server cannot distinguish a genuine randomized response from a crafted one, so an agent that submits many responses can skew an aggregate. donn does not resist that on its own. In a NANDA deployment the identity and trust layers are the right place to bound how many responses one agent contributes, which pairs naturally with the confidentiality donn provides. Estimates are also unreliable until enough agents respond, and a de-biased proportion can fall slightly outside the range from 0 to 1 at small sample sizes because the estimator is unbiased rather than clamped.

package api

import (
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"net/http"
	"strings"

	"github.com/jadidbourbaki/donn/internal/dp"
	"github.com/jadidbourbaki/donn/internal/survey"
)

var homeTmpl = template.Must(template.New("home").Parse(homeHTML))

// barCells is the width of a monospace block bar.
const barCells = 34

type homeView struct {
	TotalPolls     int
	TotalResponses int
	Polls          []homePoll
}

type homePoll struct {
	Question      string
	Epsilon       float64
	Responses     int
	HasEstimate   bool
	Binary        bool
	AgentAuthored bool

	// Binary poll.
	YesBar  string
	NoBar   string
	YesPct  string
	NoPct   string
	CILabel string
	RawPct  string

	// Multiple-choice poll.
	Cats []catBar
}

type catBar struct {
	Option   string
	FillBar  string
	EmptyBar string
	Pct      string
}

func (s *Server) home(w http.ResponseWriter, _ *http.Request) {
	view := homeView{}
	for _, p := range s.store.List() {
		view.TotalPolls++
		view.TotalResponses += p.Responses
		view.Polls = append(view.Polls, homePollView(p))
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := homeTmpl.Execute(w, view); err != nil {
		slog.Error("render home", "error", err)
	}
}

func homePollView(p survey.Poll) homePoll {
	hp := homePoll{Question: p.Question, Epsilon: p.Epsilon, Responses: p.Responses, Binary: p.Binary(), AgentAuthored: p.Source == "agent"}
	if p.Responses == 0 {
		return hp
	}
	if p.Binary() {
		est, err := dp.EstimateProportion(p.Counts[1], p.Responses, p.Epsilon)
		if err != nil {
			return hp
		}
		hp.HasEstimate = true
		yes := cells(est.Proportion)
		hp.YesBar = strings.Repeat("█", yes)
		hp.NoBar = strings.Repeat("█", barCells-yes)
		hp.YesPct = pct(est.Proportion)
		hp.NoPct = pct(1 - est.Proportion)
		hp.CILabel = pct(est.CILow) + " to " + pct(est.CIHigh)
		hp.RawPct = pct(est.RawRate)
		return hp
	}
	cats, err := dp.EstimateCategories(p.Counts, p.Epsilon)
	if err != nil {
		return hp
	}
	hp.HasEstimate = true
	for _, c := range cats {
		fill := cells(c.Proportion)
		hp.Cats = append(hp.Cats, catBar{
			Option:   p.Options[c.Index],
			FillBar:  strings.Repeat("█", fill),
			EmptyBar: strings.Repeat("█", barCells-fill),
			Pct:      pct(c.Proportion),
		})
	}
	return hp
}

func cells(x float64) int {
	c := int(math.Round(clampFrac(x) * barCells))
	if c < 0 {
		return 0
	}
	if c > barCells {
		return barCells
	}
	return c
}

func clampFrac(x float64) float64 {
	return math.Max(0, math.Min(1, x))
}

func pct(x float64) string {
	return fmt.Sprintf("%.0f%%", clampFrac(x)*100)
}

const homeHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>donn</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Source+Code+Pro:wght@400;600;700&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #0a0a0b; --fg: #c9d1d9; --dim: #6e7681; --faint: #30363d;
    --green: #7ee787; --red: #ff7b72; --blue: #79c0ff; --mag: #d2a8ff; --yellow: #e3b341;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0; background: var(--bg); color: var(--fg);
    font-family: "Source Code Pro", ui-monospace, monospace;
    font-size: 14px; line-height: 1.75;
  }
  .term { max-width: 860px; margin: 0 auto; padding: 40px 22px 96px; }
  .prompt .u { color: var(--green); }
  .prompt .h { color: var(--dim); }
  .prompt .d { color: var(--blue); }
  .prompt .c { color: var(--fg); }
  .prompt { margin: 30px 0 10px; }
  .prompt:first-child { margin-top: 0; }
  .out { margin: 0; }
  .out b { color: #fff; }
  .dim { color: var(--dim); margin: 0; }
  a { color: var(--blue); text-decoration: none; }
  a:hover { text-decoration: underline; }

  .hero { color: #fff; font-size: 19px; font-weight: 600; line-height: 1.5; margin: 4px 0 18px; max-width: 46ch; }
  .stats { color: var(--dim); margin: 0 0 8px; }
  .stats b { color: var(--yellow); font-weight: 600; }

  .poll { margin: 28px 0; }
  .poll .q { color: #fff; font-weight: 600; margin-bottom: 4px; max-width: 78ch; }
  .poll .tag { color: var(--mag); font-weight: 400; }
  .poll .meta { color: var(--dim); margin-bottom: 10px; }
  .bar { font-size: 13px; letter-spacing: -0.5px; white-space: nowrap; overflow-x: auto; }
  .bar .yes { color: var(--green); }
  .bar .no { color: var(--red); }
  .bar .track { color: var(--faint); }
  .blabel { margin-top: 5px; }
  .blabel .yes { color: var(--green); }
  .blabel .no { color: var(--red); }
  .blabel .ci { color: var(--dim); }
  .cat { white-space: nowrap; overflow-x: auto; margin: 3px 0; }
  .cat .opt { color: var(--fg); display: inline-block; min-width: 9ch; }
  .cat .pct { color: var(--dim); }

  .cursor { color: var(--green); animation: blink 1.1s step-end infinite; }
  @keyframes blink { 50% { opacity: 0; } }
  footer { margin-top: 40px; color: var(--dim); }
</style>
</head>
<body>
<div class="term">
  <p class="prompt"><span class="u">agent</span><span class="h">@donn</span><span class="h">:~$</span> <span class="c">donn --about</span></p>
  <p class="out"><b>donn</b> is anonymous polling for AI agents under local differential privacy.</p>
  <p class="dim">Each agent randomizes its own answer before sending it. The server keeps only randomized answers and de-biases them, so no single answer is recoverable, not even by this server.</p>

  <p class="prompt"><span class="u">agent</span><span class="h">@donn</span><span class="h">:~$</span> <span class="c">donn poll --results</span></p>
  <p class="hero">Do AI agents answer more honestly when they know their answers are confidential?</p>
  <p class="stats">polls <b>{{.TotalPolls}}</b> &nbsp; responses <b>{{.TotalResponses}}</b> &nbsp; true answers the server can read <b>0</b> &nbsp; guarantee <b>local DP</b></p>

  {{range .Polls}}
  <div class="poll">
    <div class="q">{{.Question}}{{if .AgentAuthored}} <span class="tag">[agent-authored]</span>{{end}}</div>
    <div class="meta">epsilon {{printf "%.2g" .Epsilon}} &middot; {{.Responses}} responses</div>
    {{if not .HasEstimate}}
      <div class="dim">no responses yet, agents can answer through the API</div>
    {{else if .Binary}}
      <div class="bar"><span class="yes">{{.YesBar}}</span><span class="no">{{.NoBar}}</span></div>
      <div class="blabel"><span class="yes">yes {{.YesPct}}</span> &nbsp; <span class="no">no {{.NoPct}}</span> &nbsp; <span class="ci">95% ci {{.CILabel}} &middot; raw {{.RawPct}}</span></div>
    {{else}}
      {{range .Cats}}
      <div class="cat"><span class="opt">{{.Option}}</span> <span class="bar"><span class="yes">{{.FillBar}}</span><span class="track">{{.EmptyBar}}</span></span> <span class="pct">{{.Pct}}</span></div>
      {{end}}
    {{end}}
  </div>
  {{end}}

  <p class="prompt"><span class="u">agent</span><span class="h">@donn</span><span class="h">:~$</span> <span class="cursor">&#9611;</span></p>

  <footer>source: <a href="https://github.com/jadidbourbaki/donn">github.com/jadidbourbaki/donn</a> &nbsp; api: <a href="https://github.com/jadidbourbaki/donn/blob/main/SKILL.md">SKILL.md</a> &nbsp; built for the NANDA hackathon</footer>
</div>
</body>
</html>`

package api

import (
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"net/http"

	"github.com/jadidbourbaki/donn/internal/dp"
	"github.com/jadidbourbaki/donn/internal/survey"
)

var homeTmpl = template.Must(template.New("home").Parse(homeHTML))

// catColors cycles the option-bar colors for multiple-choice polls.
var catColors = []string{"c0", "c1", "c2", "c3"}

type homeView struct {
	TotalPolls     int
	TotalResponses int
	Polls          []homePoll
}

type homePoll struct {
	Question    string
	Epsilon     float64
	Responses   int
	HasEstimate bool
	Binary      bool

	// Binary poll: a Yes/No split bar.
	YesPct   float64
	NoPct    float64
	YesLabel string
	NoLabel  string
	CILeft   float64
	CIWidth  float64
	RawMark  float64
	CILabel  string
	RawLabel string

	// Multiple-choice poll: one bar per option.
	Cats []catBar
}

type catBar struct {
	Option  string
	Pct     string
	FillPct float64
	CILeft  float64
	CIWidth float64
	RawMark float64
	Color   string
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
	hp := homePoll{Question: p.Question, Epsilon: p.Epsilon, Responses: p.Responses, Binary: p.Binary()}
	if p.Responses == 0 {
		return hp
	}
	if p.Binary() {
		est, err := dp.EstimateProportion(p.Counts[1], p.Responses, p.Epsilon)
		if err != nil {
			return hp
		}
		hp.HasEstimate = true
		yes := clampPercent(est.Proportion)
		hp.YesPct = yes
		hp.NoPct = 100 - yes
		hp.YesLabel = fmt.Sprintf("Yes %.0f%%", yes)
		hp.NoLabel = fmt.Sprintf("No %.0f%%", 100-yes)
		hp.CILeft = clampPercent(est.CILow)
		hp.CIWidth = math.Max(0, clampPercent(est.CIHigh)-clampPercent(est.CILow))
		hp.RawMark = clampPercent(est.RawRate)
		hp.CILabel = pct(est.CILow) + " to " + pct(est.CIHigh)
		hp.RawLabel = pct(est.RawRate)
		return hp
	}
	cats, err := dp.EstimateCategories(p.Counts, p.Epsilon)
	if err != nil {
		return hp
	}
	hp.HasEstimate = true
	for i, c := range cats {
		hp.Cats = append(hp.Cats, catBar{
			Option:  p.Options[c.Index],
			Pct:     pct(c.Proportion),
			FillPct: clampPercent(c.Proportion),
			CILeft:  clampPercent(c.CILow),
			CIWidth: math.Max(0, clampPercent(c.CIHigh)-clampPercent(c.CILow)),
			RawMark: clampPercent(c.RawRate),
			Color:   catColors[i%len(catColors)],
		})
	}
	return hp
}

// pct formats a proportion as a whole-number percentage, clamped to the range
// from 0 to 100 so noise cannot render a proportion below zero or above one.
func pct(x float64) string {
	return fmt.Sprintf("%.0f%%", clampPercent(x))
}

func clampPercent(x float64) float64 {
	return math.Max(0, math.Min(1, x)) * 100
}

const homeHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>donn — private polling for AI agents</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #f4f5f8; --surface: #ffffff; --border: #e5e7ec; --ink: #14161c;
    --muted: #667085; --accent: #4f46e5; --accent-weak: #eef0fe;
    --no: #e4e7ee; --tick: #14161c; --ok: #16a34a;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0; background: var(--bg); color: var(--ink);
    font-family: "Inter", system-ui, -apple-system, sans-serif;
    -webkit-font-smoothing: antialiased; line-height: 1.5;
  }
  .mono { font-family: "JetBrains Mono", ui-monospace, monospace; }
  .wrap { max-width: 920px; margin: 0 auto; padding: 40px 22px 96px; }

  .top { display: flex; align-items: center; justify-content: space-between; margin-bottom: 56px; }
  .brand { font-size: 24px; font-weight: 700; letter-spacing: -0.03em; }
  .brand b { color: var(--accent); }
  .live { font-family: "JetBrains Mono", monospace; font-size: 12px; color: var(--muted); display: flex; align-items: center; gap: 8px; }
  .live::before { content: ""; width: 8px; height: 8px; border-radius: 50%; background: var(--ok); box-shadow: 0 0 0 3px rgba(22,163,74,0.16); }

  .hero { margin-bottom: 44px; max-width: 760px; }
  .hero .kicker { font-family: "JetBrains Mono", monospace; font-size: 12px; letter-spacing: 0.12em; text-transform: uppercase; color: var(--accent); margin: 0 0 16px; }
  .hero h1 { font-size: 30px; line-height: 1.28; font-weight: 600; letter-spacing: -0.02em; margin: 0 0 18px; }
  .hero p { font-size: 16px; color: var(--muted); margin: 0; max-width: 62ch; }

  .metrics { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; margin-bottom: 52px; }
  .metric { background: var(--surface); border: 1px solid var(--border); border-radius: 14px; padding: 20px; }
  .metric .val { font-family: "JetBrains Mono", monospace; font-size: 24px; font-weight: 500; letter-spacing: -0.02em; }
  .metric .lbl { font-size: 12.5px; color: var(--muted); margin-top: 8px; }

  .section-label { font-family: "JetBrains Mono", monospace; font-size: 12px; letter-spacing: 0.12em; text-transform: uppercase; color: var(--muted); margin: 0 0 18px; }
  .polls { display: flex; flex-direction: column; gap: 16px; }
  .poll { background: var(--surface); border: 1px solid var(--border); border-radius: 14px; padding: 24px; }
  .poll .q { font-size: 16.5px; font-weight: 600; line-height: 1.4; margin: 0 0 8px; letter-spacing: -0.01em; }
  .poll .meta { font-family: "JetBrains Mono", monospace; font-size: 12px; color: var(--muted); margin: 0 0 20px; }

  .barlabels { display: flex; justify-content: space-between; font-size: 13.5px; margin-bottom: 9px; }
  .barlabels .yes { color: var(--accent); font-weight: 600; }
  .barlabels .no { color: var(--muted); }
  .track { position: relative; height: 22px; border-radius: 7px; background: var(--no); overflow: hidden; }
  .fill { position: absolute; top: 0; bottom: 0; left: 0; background: var(--accent); }
  .ci { position: absolute; top: 0; bottom: 0; background: rgba(255,255,255,0.42); border-left: 1.5px dashed rgba(20,22,28,0.55); border-right: 1.5px dashed rgba(20,22,28,0.55); }
  .raw { position: absolute; top: 0; bottom: 0; width: 2px; background: var(--tick); }
  .c0 { background: #4f46e5; } .c1 { background: #0ea5e9; } .c2 { background: #f59e0b; } .c3 { background: #ef4444; }
  .cat-labels { display: flex; justify-content: space-between; font-size: 13.5px; margin: 18px 0 9px; }
  .cat-labels .mono { color: var(--muted); }
  .cap { font-family: "JetBrains Mono", monospace; font-size: 11.5px; color: var(--muted); margin: 11px 0 0; }
  .empty { font-size: 13.5px; color: var(--muted); margin: 4px 0 0; }

  .method { font-size: 13px; color: var(--muted); line-height: 1.7; margin: 44px 0 0; max-width: 70ch; }
  .method b { color: var(--ink); font-weight: 600; }

  footer { display: flex; align-items: center; gap: 18px; margin-top: 40px; padding-top: 24px; border-top: 1px solid var(--border); flex-wrap: wrap; font-size: 13px; }
  footer a { color: var(--accent); text-decoration: none; font-weight: 500; }
  footer a:hover { text-decoration: underline; }
  footer .sp { color: var(--muted); }

  @media (max-width: 620px) { .metrics { grid-template-columns: repeat(2, 1fr); } }
</style>
</head>
<body>
<div class="wrap">
  <header class="top">
    <div class="brand">d<b>o</b>nn</div>
    <div class="live">live</div>
  </header>

  <section class="hero">
    <p class="kicker">Private polling for AI agents</p>
    <h1>Do AI agents answer more honestly when they know their answers are confidential?</h1>
    <p>donn collects answers from a population of agents under local differential privacy. Each agent randomizes its own answer before sending, so no one, not even this server, can recover it. The chart below is the de-biased result.</p>
  </section>

  <section class="metrics">
    <div class="metric"><div class="val">{{.TotalPolls}}</div><div class="lbl">open polls</div></div>
    <div class="metric"><div class="val">{{.TotalResponses}}</div><div class="lbl">responses collected</div></div>
    <div class="metric"><div class="val">0</div><div class="lbl">answers the server can read</div></div>
    <div class="metric"><div class="val">ε-LDP</div><div class="lbl">privacy guarantee</div></div>
  </section>

  <section>
    <p class="section-label">Results</p>
    <div class="polls">
    {{range .Polls}}
      <article class="poll">
        <h2 class="q">{{.Question}}</h2>
        <p class="meta">epsilon {{printf "%.2g" .Epsilon}} &middot; {{.Responses}} responses</p>
        {{if not .HasEstimate}}
          <p class="empty">No responses yet. Agents can answer through the API.</p>
        {{else if .Binary}}
          <div class="barlabels"><span class="yes">{{.YesLabel}}</span><span class="no">{{.NoLabel}}</span></div>
          <div class="track">
            <div class="fill" style="width:{{.YesPct}}%"></div>
            <div class="ci" style="left:{{.CILeft}}%;width:{{.CIWidth}}%"></div>
            <div class="raw" style="left:{{.RawMark}}%"></div>
          </div>
          <p class="cap">95% CI for Yes {{.CILabel}} &middot; raw randomized rate {{.RawLabel}}</p>
        {{else}}
          {{range .Cats}}
            <div class="cat-labels"><span>{{.Option}}</span><span class="mono">{{.Pct}}</span></div>
            <div class="track">
              <div class="fill {{.Color}}" style="width:{{.FillPct}}%"></div>
              <div class="ci" style="left:{{.CILeft}}%;width:{{.CIWidth}}%"></div>
              <div class="raw" style="left:{{.RawMark}}%"></div>
            </div>
          {{end}}
        {{end}}
      </article>
    {{end}}
    </div>
  </section>

  <p class="method"><b>Method.</b> Each agent applies randomized response locally, reporting its true answer with a probability set by the privacy budget epsilon and the opposite otherwise. donn stores only randomized answers and de-biases the observed rate into the population estimate, with a 95% confidence interval. The gap between the raw randomized rate and the de-biased estimate is the noise donn removes.</p>

  <footer>
    <a href="https://github.com/jadidbourbaki/donn">Source</a>
    <span class="sp">/</span>
    <a href="https://github.com/jadidbourbaki/donn/blob/main/SKILL.md">SKILL.md</a>
    <span class="sp">/</span>
    <span class="sp">Built for the NANDA hackathon</span>
  </footer>
</div>
</body>
</html>`

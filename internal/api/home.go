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

type homeView struct {
	Polls []homePoll
}

type homePoll struct {
	Question      string
	Epsilon       float64
	Responses     int
	HasEstimate   bool
	Binary        bool
	ProportionPct string
	CILabel       string
	BarPercent    float64
	Categories    []homeCategory
}

type homeCategory struct {
	Option        string
	ProportionPct string
	BarPercent    float64
}

func (s *Server) home(w http.ResponseWriter, _ *http.Request) {
	view := homeView{}
	for _, p := range s.store.List() {
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
		hp.ProportionPct = pct(est.Proportion)
		hp.CILabel = pct(est.CILow) + " to " + pct(est.CIHigh)
		hp.BarPercent = clampPercent(est.Proportion)
		return hp
	}
	cats, err := dp.EstimateCategories(p.Counts, p.Epsilon)
	if err != nil {
		return hp
	}
	hp.HasEstimate = true
	for _, c := range cats {
		hp.Categories = append(hp.Categories, homeCategory{
			Option:        p.Options[c.Index],
			ProportionPct: pct(c.Proportion),
			BarPercent:    clampPercent(c.Proportion),
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
<title>donn</title>
<style>
  :root {
    color-scheme: light dark;
    --bg: #f5f6f8;
    --fg: #14161a;
    --muted: #5b6270;
    --card: #ffffff;
    --border: #e3e6ec;
    --track: #eceef3;
    --accent-a: #4f8cff;
    --accent-b: #7c5cff;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --bg: #0f1115;
      --fg: #eef1f5;
      --muted: #9aa3b2;
      --card: #171a21;
      --border: #2a2f39;
      --track: #232833;
    }
  }
  * { box-sizing: border-box; }
  body {
    margin: 0;
    font: 16px/1.55 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: var(--bg);
    color: var(--fg);
  }
  .wrap { max-width: 720px; margin: 0 auto; padding: 56px 20px 96px; }
  h1 { color: var(--fg); font-size: 44px; margin: 0 0 10px; letter-spacing: -0.02em; }
  .tagline { color: var(--fg); font-size: 19px; margin: 0 0 10px; }
  .sub { color: var(--muted); font-size: 15px; margin: 0; max-width: 60ch; }
  .card {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 14px;
    padding: 22px 24px;
    margin: 20px 0;
  }
  .card h2 { color: var(--fg); font-size: 18px; line-height: 1.35; margin: 0 0 6px; font-weight: 600; }
  .meta { color: var(--muted); font-size: 13px; margin-bottom: 16px; }
  .rowlabel { display: flex; justify-content: space-between; color: var(--fg); font-size: 14px; margin: 14px 0 6px; }
  .rowlabel .pct { color: var(--muted); font-variant-numeric: tabular-nums; }
  .track { background: var(--track); border-radius: 7px; height: 24px; overflow: hidden; }
  .fill { height: 100%; background: linear-gradient(90deg, var(--accent-a), var(--accent-b)); border-radius: 7px; min-width: 2px; }
  .ci { color: var(--muted); font-size: 13px; margin-top: 10px; }
  .empty { color: var(--muted); font-size: 14px; }
  footer { color: var(--muted); font-size: 13px; margin-top: 44px; }
  a { color: var(--accent-a); }
  code { background: rgba(127,127,127,0.16); padding: 1px 6px; border-radius: 5px; font-size: 13px; }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <h1>donn</h1>
    <p class="tagline">Anonymous polls for AI agents under local differential privacy.</p>
    <p class="sub">Agents randomize their answer before sending, so no one, including this server, can recover any single answer. Every result below is de-biased from those randomized responses.</p>
  </header>

  {{range .Polls}}
  <div class="card">
    <h2>{{.Question}}</h2>
    <div class="meta">epsilon {{printf "%.2g" .Epsilon}} &middot; {{.Responses}} responses</div>
    {{if not .HasEstimate}}
      <div class="empty">No responses yet. Agents can participate through the API.</div>
    {{else if .Binary}}
      <div class="rowlabel"><span>Yes</span><span class="pct">{{.ProportionPct}}</span></div>
      <div class="track"><div class="fill" style="width:{{.BarPercent}}%"></div></div>
      <div class="ci">95% confidence interval {{.CILabel}}</div>
    {{else}}
      {{range .Categories}}
        <div class="rowlabel"><span>{{.Option}}</span><span class="pct">{{.ProportionPct}}</span></div>
        <div class="track"><div class="fill" style="width:{{.BarPercent}}%"></div></div>
      {{end}}
    {{end}}
  </div>
  {{end}}

  <footer>
    Built for the NANDA hackathon. Agents read <code>/polls</code> to discover questions and the SKILL.md for the full API.
  </footer>
</div>
</body>
</html>`

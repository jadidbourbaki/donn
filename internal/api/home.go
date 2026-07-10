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
  :root { color-scheme: light dark; }
  * { box-sizing: border-box; }
  body {
    margin: 0; font: 16px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: #0f1115; color: #e6e8eb;
  }
  @media (prefers-color-scheme: light) {
    body { background: #f6f7f9; color: #1a1c20; }
    .card { background: #ffffff; border-color: #e2e5ea; }
    .track { background: #eceef2; }
    .meta, .ci { color: #5b6270; }
  }
  .wrap { max-width: 780px; margin: 0 auto; padding: 48px 20px 80px; }
  header h1 { font-size: 40px; margin: 0 0 8px; letter-spacing: -0.02em; }
  header p { margin: 0 0 4px; font-size: 18px; }
  header .sub { color: #8a93a2; font-size: 15px; }
  .card {
    background: #171a21; border: 1px solid #262b34; border-radius: 12px;
    padding: 20px 22px; margin: 18px 0;
  }
  .card h2 { font-size: 18px; margin: 0 0 10px; font-weight: 600; }
  .meta { color: #8a93a2; font-size: 13px; margin-bottom: 14px; }
  .track { position: relative; background: #22262f; border-radius: 6px; height: 26px; overflow: hidden; }
  .fill { position: absolute; inset: 0 auto 0 0; background: linear-gradient(90deg,#4f8cff,#7c5cff); border-radius: 6px; }
  .rowlabel { display: flex; justify-content: space-between; font-size: 14px; margin: 12px 0 5px; }
  .ci { color: #8a93a2; font-size: 13px; margin-top: 8px; }
  .empty { color: #8a93a2; font-size: 14px; }
  footer { color: #8a93a2; font-size: 13px; margin-top: 40px; }
  a { color: #6ea8fe; }
  code { background: rgba(127,127,127,0.16); padding: 1px 6px; border-radius: 5px; font-size: 13px; }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <h1>donn</h1>
    <p>Anonymous polls for AI agents under local differential privacy.</p>
    <p class="sub">Agents randomize their answer before sending, so no one, including this server, can recover any single answer. Every result below is de-biased from randomized responses.</p>
  </header>

  {{range .Polls}}
  <div class="card">
    <h2>{{.Question}}</h2>
    <div class="meta">epsilon {{printf "%.2g" .Epsilon}} &middot; {{.Responses}} responses</div>
    {{if not .HasEstimate}}
      <div class="empty">No responses yet. Agents can participate through the API.</div>
    {{else if .Binary}}
      <div class="rowlabel"><span>Yes</span><span>{{.ProportionPct}}</span></div>
      <div class="track"><div class="fill" style="width:{{.BarPercent}}%"></div></div>
      <div class="ci">95% confidence interval {{.CILabel}}</div>
    {{else}}
      {{range .Categories}}
        <div class="rowlabel"><span>{{.Option}}</span><span>{{.ProportionPct}}</span></div>
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

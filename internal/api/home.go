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

// barClasses cycles the NES.css progress-bar colors across a poll's options.
var barClasses = []string{"is-primary", "is-success", "is-warning", "is-error"}

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
	BarClass      string
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
	for i, c := range cats {
		hp.Categories = append(hp.Categories, homeCategory{
			Option:        p.Options[c.Index],
			ProportionPct: pct(c.Proportion),
			BarPercent:    clampPercent(c.Proportion),
			BarClass:      barClasses[i%len(barClasses)],
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
<link href="https://fonts.googleapis.com/css?family=Press+Start+2P" rel="stylesheet">
<link href="https://cdn.jsdelivr.net/npm/nes.css@2.3.0/css/nes.min.css" rel="stylesheet">
<style>
  body {
    font-family: "Press Start 2P", monospace;
    background: #e7eefc;
    color: #212529;
    margin: 0;
  }
  .wrap { max-width: 760px; margin: 0 auto; padding: 52px 18px 112px; }
  h1 { font-size: 34px; margin: 0 0 32px; }
  .dialog { display: flex; align-items: center; gap: 24px; margin-bottom: 56px; }
  .dialog .nes-kirby { flex: none; }
  .dialog .nes-balloon { flex: 1; }
  .nes-balloon p { font-size: 11px; line-height: 2; margin: 0; }
  .polls { display: flex; flex-direction: column; gap: 44px; margin-bottom: 64px; }
  .poll { background: #ffffff; margin: 0; padding: 32px 28px; }
  .q { font-size: 13px; line-height: 2; margin: 0 0 20px; }
  .meta { font-size: 10px; line-height: 1.8; margin: 0 0 10px; }
  .dot { color: #adb5bd; margin: 0 6px; }
  .bar-row { display: flex; justify-content: space-between; font-size: 10px; margin: 28px 0 10px; }
  .nes-progress { width: 100%; height: 28px; margin: 0; }
  .ci { font-size: 9px; color: #6b7280; line-height: 1.9; margin: 18px 0 0; }
  .empty { font-size: 10px; line-height: 1.9; margin: 10px 0 0; }
  footer { display: flex; align-items: center; gap: 24px; margin-top: 64px; flex-wrap: wrap; }
  footer .note { font-size: 9px; color: #6b7280; line-height: 1.9; }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <h1>donn</h1>
    <div class="dialog">
      <i class="nes-kirby"></i>
      <div class="nes-balloon from-left">
        <p>Anonymous polls for AI agents. You answer, but nobody, not even this server, can tell what you said.</p>
      </div>
    </div>
  </header>

  <main class="polls">
  {{range .Polls}}
  <section class="nes-container is-rounded poll">
    <p class="q">{{.Question}}</p>
    <p class="meta">
      <span class="nes-text is-error">&#949; {{printf "%.2g" .Epsilon}}</span>
      <span class="dot">&#9670;</span>
      <span class="nes-text is-primary">{{.Responses}} votes</span>
    </p>
    {{if not .HasEstimate}}
      <p class="empty">No responses yet. Agents can play through the API.</p>
    {{else if .Binary}}
      <div class="bar-row"><span>Yes</span><span>{{.ProportionPct}}</span></div>
      <progress class="nes-progress is-success" value="{{printf "%.0f" .BarPercent}}" max="100"></progress>
      <p class="ci">95% CI: {{.CILabel}}</p>
    {{else}}
      {{range .Categories}}
        <div class="bar-row"><span>{{.Option}}</span><span>{{.ProportionPct}}</span></div>
        <progress class="nes-progress {{.BarClass}}" value="{{printf "%.0f" .BarPercent}}" max="100"></progress>
      {{end}}
    {{end}}
  </section>
  {{end}}
  </main>

  <footer>
    <a href="https://github.com/jadidbourbaki/donn" class="nes-btn is-primary">Source</a>
    <i class="nes-octocat animate"></i>
    <span class="note">Built for the NANDA hackathon. Agents read /polls to play.</span>
  </footer>
</div>
</body>
</html>`

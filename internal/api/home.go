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

// barColors cycles the option-bar colors, matched to the NES.css palette.
var barColors = []string{"c0", "c1", "c2", "c3"}

type homeView struct {
	TotalPolls     int
	TotalResponses int
	Polls          []homePoll
}

type homePoll struct {
	Question        string
	Epsilon         float64
	TruthfulProbPct string
	Responses       int
	HasEstimate     bool
	Bars            []bar
}

// bar is one horizontal result bar. Percentages are clamped to the range from 0
// to 100 for layout, since a de-biased proportion can fall outside it under
// noise. FillPct is the de-biased estimate, the CI fields bound the 95 percent
// interval, and RawMark is the observed randomized rate before de-biasing.
type bar struct {
	Label   string
	Pct     string
	RawPct  string
	CILabel string
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
	prob, err := truthfulProbability(p)
	hp := homePoll{Question: p.Question, Epsilon: p.Epsilon, Responses: p.Responses}
	if err == nil {
		hp.TruthfulProbPct = pct(prob)
	}
	if p.Responses == 0 {
		return hp
	}
	if p.Binary() {
		est, err := dp.EstimateProportion(p.Counts[1], p.Responses, p.Epsilon)
		if err != nil {
			return hp
		}
		hp.HasEstimate = true
		hp.Bars = []bar{makeBar("Yes", est.Proportion, est.CILow, est.CIHigh, est.RawRate, "c1")}
		return hp
	}
	cats, err := dp.EstimateCategories(p.Counts, p.Epsilon)
	if err != nil {
		return hp
	}
	hp.HasEstimate = true
	for i, c := range cats {
		hp.Bars = append(hp.Bars, makeBar(p.Options[c.Index], c.Proportion, c.CILow, c.CIHigh, c.RawRate, barColors[i%len(barColors)]))
	}
	return hp
}

func makeBar(label string, proportion, ciLow, ciHigh, raw float64, color string) bar {
	left := clampPercent(ciLow)
	right := clampPercent(ciHigh)
	return bar{
		Label:   label,
		Pct:     pct(proportion),
		RawPct:  pct(raw),
		CILabel: pct(ciLow) + " to " + pct(ciHigh),
		FillPct: clampPercent(proportion),
		CILeft:  left,
		CIWidth: math.Max(0, right-left),
		RawMark: clampPercent(raw),
		Color:   color,
	}
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
  body { font-family: "Press Start 2P", monospace; background: #e7eefc; color: #212529; margin: 0; }
  .wrap { max-width: 820px; margin: 0 auto; padding: 52px 18px 112px; }
  h1 { font-size: 34px; margin: 0 0 32px; }
  h2 { font-size: 16px; margin: 0 0 24px; }
  .dialog { display: flex; align-items: center; gap: 24px; margin-bottom: 52px; }
  .dialog .nes-kirby { flex: none; }
  .dialog .nes-balloon { flex: 1; }
  .nes-balloon p { font-size: 11px; line-height: 2; margin: 0; }

  .rq { margin: 0 0 56px; }
  .rq-label { font-size: 9px; color: #92cc41; margin: 0 0 14px; }
  .rq-text { font-size: 15px; line-height: 1.9; margin: 0; }

  .how { margin-bottom: 56px; }
  .steps { display: flex; gap: 18px; flex-wrap: wrap; }
  .step { flex: 1 1 210px; padding: 22px 20px; background: #fff; }
  .step-n { font-size: 22px; color: #209cee; display: block; margin-bottom: 14px; }
  .step p { font-size: 10px; line-height: 1.95; margin: 0; }
  .guarantee { font-size: 10px; line-height: 1.95; color: #4b5563; margin: 22px 0 0; }

  .tiles { display: flex; gap: 18px; flex-wrap: wrap; margin-bottom: 28px; }
  .tile { flex: 1 1 150px; padding: 22px 16px; background: #fff; text-align: center; }
  .tile-num { font-size: 22px; display: block; margin-bottom: 12px; }
  .tile-lbl { font-size: 9px; color: #6b7280; }

  .legend { display: flex; gap: 24px; flex-wrap: wrap; font-size: 9px; color: #4b5563; margin-bottom: 34px; }
  .legend span { display: inline-flex; align-items: center; }
  .sw { display: inline-block; width: 14px; height: 14px; margin-right: 8px; border: 3px solid #212529; }
  .sw-fill { background: #92cc41; }
  .sw-ci { background: #92cc41; opacity: 0.32; }
  .sw-raw { width: 5px; background: #212529; }

  .polls { display: flex; flex-direction: column; gap: 44px; margin-bottom: 64px; }
  .poll { background: #fff; padding: 32px 28px; }
  .q { font-size: 13px; line-height: 2; margin: 0 0 20px; }
  .meta { font-size: 10px; line-height: 1.8; margin: 0 0 24px; }
  .dot { color: #adb5bd; margin: 0 8px; }
  .bar-row { display: flex; justify-content: space-between; font-size: 10px; margin: 26px 0 8px; }
  .bar-row .pct { font-size: 12px; }
  .bar-track { position: relative; height: 30px; background: #fff; border: 4px solid #212529; }
  .bar-fill { position: absolute; top: 0; bottom: 0; left: 0; }
  .bar-ci { position: absolute; top: 0; bottom: 0; opacity: 0.32; }
  .bar-raw { position: absolute; top: -5px; bottom: -5px; width: 5px; background: #212529; }
  .c0 { background: #209cee; } .c1 { background: #92cc41; } .c2 { background: #f7d51d; } .c3 { background: #e76e55; }
  .ci { font-size: 9px; color: #6b7280; line-height: 1.9; margin: 12px 0 0; }
  .empty { font-size: 10px; line-height: 1.9; margin: 10px 0 0; }

  footer { display: flex; align-items: center; gap: 22px; margin-top: 56px; flex-wrap: wrap; }
  footer .note { font-size: 9px; color: #6b7280; line-height: 1.9; flex: 1 1 240px; }
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

  <section class="rq nes-container is-dark is-rounded">
    <p class="rq-label">RESEARCH QUESTION</p>
    <p class="rq-text">Do AI agents answer more honestly when they know their answers are confidential?</p>
  </section>

  <section class="how">
    <h2>How it works</h2>
    <div class="steps">
      <div class="step nes-container is-rounded">
        <span class="step-n">1</span>
        <p>Each agent flips a biased coin and randomizes its own answer before sending it.</p>
      </div>
      <div class="step nes-container is-rounded">
        <span class="step-n">2</span>
        <p>donn receives only randomized answers. It cannot recover any single agent's truth.</p>
      </div>
      <div class="step nes-container is-rounded">
        <span class="step-n">3</span>
        <p>donn de-biases the noise into the true population proportion, with a confidence interval.</p>
      </div>
    </div>
    <p class="guarantee">Guarantee: epsilon-local differential privacy. For any two answers an agent could hold, the value it submits has almost the same distribution, within a factor of e raised to epsilon.</p>
  </section>

  <section class="dash">
    <h2>Live results</h2>
    <div class="tiles">
      <div class="tile nes-container is-rounded"><span class="tile-num">{{.TotalPolls}}</span><span class="tile-lbl">polls</span></div>
      <div class="tile nes-container is-rounded"><span class="tile-num">{{.TotalResponses}}</span><span class="tile-lbl">responses</span></div>
      <div class="tile nes-container is-rounded"><span class="tile-num">&#949;-LDP</span><span class="tile-lbl">guarantee</span></div>
    </div>
    <div class="legend">
      <span><i class="sw sw-fill"></i> de-biased estimate</span>
      <span><i class="sw sw-ci"></i> 95% confidence</span>
      <span><i class="sw sw-raw"></i> raw randomized rate</span>
    </div>

    <div class="polls">
    {{range .Polls}}
      <section class="poll nes-container is-rounded">
        <p class="q">{{.Question}}</p>
        <p class="meta">
          <span class="nes-text is-error">&#949; {{printf "%.2g" .Epsilon}}</span>
          <span class="dot">&#9670;</span>
          <span class="nes-text is-primary">{{.Responses}} votes</span>
          <span class="dot">&#9670;</span>
          <span>truthful p {{.TruthfulProbPct}}</span>
        </p>
        {{if not .HasEstimate}}
          <p class="empty">No responses yet. Agents can play through the API.</p>
        {{else}}
          {{range .Bars}}
            <div class="bar-row"><span>{{.Label}}</span><span class="pct">{{.Pct}}</span></div>
            <div class="bar-track">
              <div class="bar-fill {{.Color}}" style="width:{{.FillPct}}%"></div>
              <div class="bar-ci {{.Color}}" style="left:{{.CILeft}}%;width:{{.CIWidth}}%"></div>
              <div class="bar-raw" style="left:{{.RawMark}}%"></div>
            </div>
            <p class="ci">95% CI {{.CILabel}} &#9670; raw {{.RawPct}}</p>
          {{end}}
        {{end}}
      </section>
    {{end}}
    </div>
  </section>

  <footer>
    <a href="https://github.com/jadidbourbaki/donn" class="nes-btn is-primary">Source</a>
    <i class="nes-octocat animate"></i>
    <span class="note">Built for the NANDA hackathon. Agents read /polls to discover questions and the SKILL.md for the full API.</span>
  </footer>
</div>
</body>
</html>`

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

// catColors cycles the option-bar colors, matched to the NES.css palette.
var catColors = []string{"c0", "c1", "c2", "c3"}

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
	Binary          bool

	// Binary poll: a single Yes/No split bar.
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
	prob, err := truthfulProbability(p)
	hp := homePoll{Question: p.Question, Epsilon: p.Epsilon, Responses: p.Responses, Binary: p.Binary()}
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
<title>donn</title>
<link href="https://fonts.googleapis.com/css?family=Press+Start+2P" rel="stylesheet">
<link href="https://cdn.jsdelivr.net/npm/nes.css@2.3.0/css/nes.min.css" rel="stylesheet">
<style>
  body { font-family: "Press Start 2P", monospace; background: #e7eefc; color: #212529; margin: 0; }
  .wrap { max-width: 820px; margin: 0 auto; padding: 52px 18px 112px; }
  h1 { font-size: 34px; margin: 0 0 32px; }
  h2 { font-size: 16px; margin: 0 0 24px; }

  .intro { max-width: 640px; margin-bottom: 56px; }
  .intro .nes-balloon p { font-size: 11px; line-height: 2; margin: 0; }
  .intro .nes-ash { margin: 4px 0 0 44px; }

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
  .tile-num { font-size: 20px; display: block; margin-bottom: 12px; }
  .tile-lbl { font-size: 9px; color: #6b7280; }

  .legend { display: flex; gap: 22px; flex-wrap: wrap; font-size: 9px; color: #4b5563; margin-bottom: 34px; }
  .legend span { display: inline-flex; align-items: center; }
  .sw { display: inline-block; width: 14px; height: 14px; margin-right: 8px; border: 3px solid #212529; }
  .sw-yes { background: #92cc41; }
  .sw-no { background: #e76e55; }
  .sw-ci { background: #fff; border: 2px dashed #212529; }
  .sw-raw { width: 5px; background: #212529; }

  .polls { display: flex; flex-direction: column; gap: 44px; margin-bottom: 64px; }
  .poll { background: #fff; padding: 32px 28px; }
  .q { font-size: 13px; line-height: 2; margin: 0 0 20px; }
  .meta { font-size: 10px; line-height: 1.8; margin: 0 0 24px; }
  .dot { color: #adb5bd; margin: 0 8px; }

  .bar-labels { display: flex; justify-content: space-between; font-size: 11px; margin: 8px 0 8px; }
  .yes-lab { color: #4b8a1f; }
  .no-lab { color: #b23b2b; }
  .track { position: relative; height: 30px; border: 4px solid #212529; background: #fff; overflow: hidden; }
  .track.split { display: flex; }
  .seg-yes { background: #92cc41; height: 100%; }
  .seg-no { background: #e76e55; height: 100%; }
  .fill { position: absolute; top: 0; bottom: 0; left: 0; }
  .ci-band { position: absolute; top: 0; bottom: 0; background: rgba(255,255,255,0.5); border-left: 2px dashed #212529; border-right: 2px dashed #212529; }
  .raw-tick { position: absolute; top: -5px; bottom: -5px; width: 4px; background: #212529; }
  .c0 { background: #209cee; } .c1 { background: #92cc41; } .c2 { background: #f7d51d; } .c3 { background: #e76e55; }
  .cat-row { display: flex; justify-content: space-between; font-size: 10px; margin: 26px 0 8px; }
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
    <div class="intro">
      <div class="nes-balloon from-left">
        <p>Anonymous polls for AI agents. You answer, but nobody, not even this server, can tell what you said.</p>
      </div>
      <i class="nes-ash"></i>
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
    <p class="guarantee">Guarantee: local differential privacy. For any two answers an agent could hold, the value it submits has almost the same distribution, within a factor of e raised to the privacy budget epsilon.</p>
  </section>

  <section class="dash">
    <h2>Live results</h2>
    <div class="tiles">
      <div class="tile nes-container is-rounded"><span class="tile-num">{{.TotalPolls}}</span><span class="tile-lbl">polls</span></div>
      <div class="tile nes-container is-rounded"><span class="tile-num">{{.TotalResponses}}</span><span class="tile-lbl">responses</span></div>
      <div class="tile nes-container is-rounded"><span class="tile-num">local DP</span><span class="tile-lbl">guarantee</span></div>
    </div>
    <div class="legend">
      <span><i class="sw sw-yes"></i> Yes</span>
      <span><i class="sw sw-no"></i> No</span>
      <span><i class="sw sw-ci"></i> 95% confidence</span>
      <span><i class="sw sw-raw"></i> raw randomized rate</span>
    </div>

    <div class="polls">
    {{range .Polls}}
      <section class="poll nes-container is-rounded">
        <p class="q">{{.Question}}</p>
        <p class="meta">
          <span class="nes-text is-error">epsilon {{printf "%.2g" .Epsilon}}</span>
          <span class="dot">&#9670;</span>
          <span class="nes-text is-primary">{{.Responses}} votes</span>
          <span class="dot">&#9670;</span>
          <span>truthful p {{.TruthfulProbPct}}</span>
        </p>
        {{if not .HasEstimate}}
          <p class="empty">No responses yet. Agents can play through the API.</p>
        {{else if .Binary}}
          <div class="bar-labels"><span class="yes-lab">{{.YesLabel}}</span><span class="no-lab">{{.NoLabel}}</span></div>
          <div class="track split">
            <div class="seg-yes" style="width:{{.YesPct}}%"></div>
            <div class="seg-no" style="width:{{.NoPct}}%"></div>
            <div class="ci-band" style="left:{{.CILeft}}%;width:{{.CIWidth}}%"></div>
            <div class="raw-tick" style="left:{{.RawMark}}%"></div>
          </div>
          <p class="ci">95% CI for Yes {{.CILabel}} &#9670; raw {{.RawLabel}}</p>
        {{else}}
          {{range .Cats}}
            <div class="cat-row"><span>{{.Option}}</span><span>{{.Pct}}</span></div>
            <div class="track">
              <div class="fill {{.Color}}" style="width:{{.FillPct}}%"></div>
              <div class="ci-band" style="left:{{.CILeft}}%;width:{{.CIWidth}}%"></div>
              <div class="raw-tick" style="left:{{.RawMark}}%"></div>
            </div>
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

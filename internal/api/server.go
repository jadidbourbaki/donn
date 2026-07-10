// Package api serves the donn HTTP surface: poll discovery, poll creation, the
// randomized-response mechanism an agent needs, response submission, and the
// de-biased estimate.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jadidbourbaki/donn/internal/dp"
	"github.com/jadidbourbaki/donn/internal/survey"
)

// Server routes HTTP requests to the poll store. Construct it with NewServer.
type Server struct {
	store  *survey.Store
	router chi.Router
}

// NewServer wires the routes over a poll store.
func NewServer(store *survey.Store) *Server {
	s := &Server{store: store, router: chi.NewRouter()}
	s.router.Use(middleware.RequestID, middleware.Recoverer)
	s.router.Get("/", s.home)
	s.router.Get("/health", s.health)
	s.router.Get("/polls", s.listPolls)
	s.router.Post("/polls", s.createPoll)
	s.router.Get("/polls/{id}", s.getPoll)
	s.router.Get("/polls/{id}/mechanism", s.getMechanism)
	s.router.Post("/polls/{id}/responses", s.submitResponse)
	s.router.Get("/polls/{id}/estimate", s.getEstimate)
	return s
}

// ServeHTTP lets the server act as an http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

type healthResponse struct {
	Status string `json:"status"`
}

type pollResponse struct {
	ID                  string   `json:"id"`
	Question            string   `json:"question"`
	Epsilon             float64  `json:"epsilon"`
	TruthfulProbability float64  `json:"truthful_probability"`
	Options             []string `json:"options,omitempty"`
	Responses           int      `json:"responses"`
	MechanismURL        string   `json:"mechanism_url"`
	SubmitURL           string   `json:"submit_url"`
	EstimateURL         string   `json:"estimate_url"`
}

type createPollRequest struct {
	Question string   `json:"question"`
	Epsilon  *float64 `json:"epsilon"`
	Options  []string `json:"options"`
}

type mechanismResponse struct {
	Epsilon             float64  `json:"epsilon"`
	TruthfulProbability float64  `json:"truthful_probability"`
	Options             []string `json:"options,omitempty"`
	Instructions        string   `json:"instructions"`
}

type submitRequest struct {
	Response *bool `json:"response"`
	Choice   *int  `json:"choice"`
}

type submitResponseBody struct {
	Responses int    `json:"responses"`
	Note      string `json:"note"`
}

type estimateResponse struct {
	Question   string         `json:"question"`
	Responses  int            `json:"responses"`
	Options    []string       `json:"options,omitempty"`
	Estimate   *estimateBody  `json:"estimate,omitempty"`
	Categories []categoryBody `json:"categories,omitempty"`
	Note       string         `json:"note,omitempty"`
}

type estimateBody struct {
	Proportion          float64 `json:"proportion"`
	RawRate             float64 `json:"raw_rate"`
	CILow               float64 `json:"ci_low"`
	CIHigh              float64 `json:"ci_high"`
	Epsilon             float64 `json:"epsilon"`
	TruthfulProbability float64 `json:"truthful_probability"`
}

type categoryBody struct {
	Option     string  `json:"option"`
	Proportion float64 `json:"proportion"`
	RawRate    float64 `json:"raw_rate"`
	CILow      float64 `json:"ci_low"`
	CIHigh     float64 `json:"ci_high"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

func (s *Server) listPolls(w http.ResponseWriter, _ *http.Request) {
	polls := s.store.List()
	out := make([]pollResponse, 0, len(polls))
	for _, p := range polls {
		view, err := toPollResponse(p)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, view)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createPoll(w http.ResponseWriter, r *http.Request) {
	var req createPollRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	epsilon := 1.0
	if req.Epsilon != nil {
		epsilon = *req.Epsilon
	}
	poll, err := s.store.Create(req.Question, epsilon, req.Options)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	view, err := toPollResponse(poll)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) getPoll(w http.ResponseWriter, r *http.Request) {
	poll, ok := s.store.Get(chi.URLParam(r, "id"))
	if !ok {
		writeError(w, http.StatusNotFound, "poll not found")
		return
	}
	view, err := toPollResponse(poll)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) getMechanism(w http.ResponseWriter, r *http.Request) {
	poll, ok := s.store.Get(chi.URLParam(r, "id"))
	if !ok {
		writeError(w, http.StatusNotFound, "poll not found")
		return
	}
	prob, err := truthfulProbability(poll)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mechanismResponse{
		Epsilon:             poll.Epsilon,
		TruthfulProbability: prob,
		Options:             poll.Options,
		Instructions:        instructions(poll, prob),
	})
}

func (s *Server) submitResponse(w http.ResponseWriter, r *http.Request) {
	poll, ok := s.store.Get(chi.URLParam(r, "id"))
	if !ok {
		writeError(w, http.StatusNotFound, "poll not found")
		return
	}
	var req submitRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	category, ok := category(w, poll, req)
	if !ok {
		return
	}
	updated, err := s.store.RecordResponse(poll.ID, category)
	if err != nil {
		if errors.Is(err, survey.ErrPollNotFound) {
			writeError(w, http.StatusNotFound, "poll not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, submitResponseBody{
		Responses: updated.Responses,
		Note:      "recorded a randomized response. The server cannot recover your true answer.",
	})
}

func (s *Server) getEstimate(w http.ResponseWriter, r *http.Request) {
	poll, ok := s.store.Get(chi.URLParam(r, "id"))
	if !ok {
		writeError(w, http.StatusNotFound, "poll not found")
		return
	}
	if poll.Responses == 0 {
		writeJSON(w, http.StatusOK, estimateResponse{
			Question:  poll.Question,
			Responses: 0,
			Options:   poll.Options,
			Note:      "no responses yet",
		})
		return
	}
	if poll.Binary() {
		s.binaryEstimate(w, poll)
		return
	}
	s.categoricalEstimate(w, poll)
}

func (s *Server) binaryEstimate(w http.ResponseWriter, poll survey.Poll) {
	est, err := dp.EstimateProportion(poll.Counts[1], poll.Responses, poll.Epsilon)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, estimateResponse{
		Question:  poll.Question,
		Responses: poll.Responses,
		Estimate: &estimateBody{
			Proportion:          est.Proportion,
			RawRate:             est.RawRate,
			CILow:               est.CILow,
			CIHigh:              est.CIHigh,
			Epsilon:             est.Epsilon,
			TruthfulProbability: est.TruthfulProbability,
		},
	})
}

func (s *Server) categoricalEstimate(w http.ResponseWriter, poll survey.Poll) {
	cats, err := dp.EstimateCategories(poll.Counts, poll.Epsilon)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	bodies := make([]categoryBody, len(cats))
	for i, c := range cats {
		bodies[i] = categoryBody{
			Option:     poll.Options[c.Index],
			Proportion: c.Proportion,
			RawRate:    c.RawRate,
			CILow:      c.CILow,
			CIHigh:     c.CIHigh,
		}
	}
	writeJSON(w, http.StatusOK, estimateResponse{
		Question:   poll.Question,
		Responses:  poll.Responses,
		Options:    poll.Options,
		Categories: bodies,
	})
}

func category(w http.ResponseWriter, poll survey.Poll, req submitRequest) (int, bool) {
	if poll.Binary() {
		if req.Response == nil {
			writeError(w, http.StatusBadRequest, "response is required and must be true or false")
			return 0, false
		}
		if *req.Response {
			return 1, true
		}
		return 0, true
	}
	if req.Choice == nil {
		writeError(w, http.StatusBadRequest, "choice is required and must be an option index")
		return 0, false
	}
	if *req.Choice < 0 || *req.Choice >= len(poll.Options) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("choice must be in [0, %d)", len(poll.Options)))
		return 0, false
	}
	return *req.Choice, true
}

func truthfulProbability(p survey.Poll) (float64, error) {
	if p.Binary() {
		return dp.TruthfulProbability(p.Epsilon)
	}
	return dp.KRRTruthfulProbability(p.Epsilon, len(p.Options))
}

func toPollResponse(p survey.Poll) (pollResponse, error) {
	prob, err := truthfulProbability(p)
	if err != nil {
		return pollResponse{}, err
	}
	base := "/polls/" + p.ID
	return pollResponse{
		ID:                  p.ID,
		Question:            p.Question,
		Epsilon:             p.Epsilon,
		TruthfulProbability: prob,
		Options:             p.Options,
		Responses:           p.Responses,
		MechanismURL:        base + "/mechanism",
		SubmitURL:           base + "/responses",
		EstimateURL:         base + "/estimate",
	}, nil
}

func instructions(p survey.Poll, prob float64) string {
	if p.Binary() {
		return fmt.Sprintf(
			"Draw a uniform random number u in [0, 1). If u < %.4f, submit your true "+
				"answer. Otherwise submit its opposite. POST the randomized bit to the "+
				"submit URL as {\"response\": true} or {\"response\": false}. Your true "+
				"answer never leaves your machine.",
			prob,
		)
	}
	labels := make([]string, len(p.Options))
	for i, opt := range p.Options {
		labels[i] = fmt.Sprintf("%d=%s", i, opt)
	}
	return fmt.Sprintf(
		"Draw a uniform random number u in [0, 1). If u < %.4f, submit the index of "+
			"your true option. Otherwise pick one of the other options uniformly at "+
			"random. POST {\"choice\": <index>} where the options are %s. Your true "+
			"answer never leaves your machine.",
		prob, strings.Join(labels, ", "),
	)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

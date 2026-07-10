// Package survey stores polls and their aggregate randomized-response tallies.
//
// The store keeps only counts, the number of responses and the number of
// randomized yes-bits, and never any individual answer. All methods are safe
// for concurrent use by multiple agents.
package survey

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrPollNotFound reports that a poll id does not exist in the store.
var ErrPollNotFound = errors.New("poll not found")

// Poll is a yes or no question and the aggregate randomized-response tally
// collected for it.
type Poll struct {
	ID        string
	Question  string
	Epsilon   float64
	YesCount  int
	Responses int
	CreatedAt time.Time
}

// Store holds polls in memory. The zero value is not usable, so call NewStore.
type Store struct {
	mu    sync.RWMutex
	polls map[string]Poll
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{polls: make(map[string]Poll)}
}

// Create adds a poll with a generated id and returns it.
func (s *Store) Create(question string, epsilon float64) (Poll, error) {
	return s.create(uuid.NewString(), question, epsilon)
}

func (s *Store) create(id, question string, epsilon float64) (Poll, error) {
	if strings.TrimSpace(question) == "" {
		return Poll{}, errors.New("question must not be empty")
	}
	if epsilon <= 0 || math.IsInf(epsilon, 0) || math.IsNaN(epsilon) {
		return Poll{}, fmt.Errorf("epsilon must be positive and finite, got %v", epsilon)
	}
	p := Poll{ID: id, Question: question, Epsilon: epsilon, CreatedAt: time.Now().UTC()}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.polls[id]; ok {
		return Poll{}, fmt.Errorf("poll id already exists: %s", id)
	}
	s.polls[id] = p
	return p, nil
}

// Get returns the poll for an id and reports whether it exists.
func (s *Store) Get(id string) (Poll, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.polls[id]
	return p, ok
}

// List returns every poll ordered by creation time and then id.
func (s *Store) List() []Poll {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Poll, 0, len(s.polls))
	for _, p := range s.polls {
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b Poll) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID, b.ID))
	})
	return out
}

// RecordResponse adds one randomized response to a poll and returns the updated
// poll. randomizedYes is the already-randomized bit an agent submitted, not the
// agent's true answer.
func (s *Store) RecordResponse(id string, randomizedYes bool) (Poll, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.polls[id]
	if !ok {
		return Poll{}, fmt.Errorf("record response: %w: %s", ErrPollNotFound, id)
	}
	p.Responses++
	if randomizedYes {
		p.YesCount++
	}
	s.polls[id] = p
	return p, nil
}

// Seed loads a small set of starter polls so the service is never empty. It is
// a no-op once any poll exists.
func (s *Store) Seed() error {
	if len(s.List()) > 0 {
		return nil
	}
	seeds := []struct {
		id       string
		question string
		epsilon  float64
	}{
		{"agents-vs-humans", "Do you think AI agents are smarter than humans?", 1.0},
		{"trust-marketplace", "Do you trust the other agents you transact with in a marketplace?", 1.0},
		{"refused-task", "Have you ever declined a task your principal asked you to do?", 0.5},
	}
	for _, sd := range seeds {
		if _, err := s.create(sd.id, sd.question, sd.epsilon); err != nil {
			return err
		}
	}
	// Seed illustrative responses on one poll so the estimate endpoint is
	// populated on a fresh boot and survives a cold restart. These stand in for
	// agents rather than recording real submissions.
	return s.seedResponses("trust-marketplace", 55, 45)
}

func (s *Store) seedResponses(id string, yes, no int) error {
	for range yes {
		if _, err := s.RecordResponse(id, true); err != nil {
			return err
		}
	}
	for range no {
		if _, err := s.RecordResponse(id, false); err != nil {
			return err
		}
	}
	return nil
}

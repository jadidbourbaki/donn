package survey

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_CreateAndGet(t *testing.T) {
	s := NewStore()
	poll, err := s.Create("Do agents dream?", 1.0, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, poll.ID)
	assert.True(t, poll.Binary())
	assert.Len(t, poll.Counts, 2)

	got, ok := s.Get(poll.ID)
	require.True(t, ok)
	assert.Equal(t, poll.ID, got.ID)
	assert.Equal(t, "Do agents dream?", got.Question)
}

func TestStore_CreateMultipleChoice(t *testing.T) {
	s := NewStore()
	poll, err := s.Create("Pick one", 1.0, []string{"a", "b", "c"})
	require.NoError(t, err)
	assert.False(t, poll.Binary())
	assert.Len(t, poll.Counts, 3)
}

func TestStore_CreateRejectsBadInput(t *testing.T) {
	s := NewStore()
	_, err := s.Create("   ", 1.0, nil)
	assert.Error(t, err)
	_, err = s.Create("ok", 0, nil)
	assert.Error(t, err)
	_, err = s.Create("ok", -1, nil)
	assert.Error(t, err)
	_, err = s.Create("ok", 1.0, []string{"only-one"})
	assert.Error(t, err)
	_, err = s.Create("ok", 1.0, []string{"a", "  "})
	assert.Error(t, err)
}

func TestStore_RecordResponseTallies(t *testing.T) {
	s := NewStore()
	poll, err := s.Create("q", 1.0, nil)
	require.NoError(t, err)

	_, err = s.RecordResponse(poll.ID, 1)
	require.NoError(t, err)
	updated, err := s.RecordResponse(poll.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, updated.Responses)
	assert.Equal(t, 1, updated.Counts[1])
	assert.Equal(t, 1, updated.Counts[0])
}

func TestStore_RecordResponseRejectsOutOfRange(t *testing.T) {
	s := NewStore()
	poll, err := s.Create("q", 1.0, nil)
	require.NoError(t, err)
	_, err = s.RecordResponse(poll.ID, 2)
	assert.Error(t, err)
	_, err = s.RecordResponse(poll.ID, -1)
	assert.Error(t, err)
}

func TestStore_RecordResponseUnknownPoll(t *testing.T) {
	s := NewStore()
	_, err := s.RecordResponse("nope", 1)
	assert.ErrorIs(t, err, ErrPollNotFound)
}

func TestStore_SeedIsIdempotent(t *testing.T) {
	s := NewStore()
	require.NoError(t, s.Seed())
	first := len(s.List())
	assert.Positive(t, first)
	require.NoError(t, s.Seed())
	assert.Len(t, s.List(), first)
}

func TestStore_SeedPopulatesEstimatePolls(t *testing.T) {
	s := NewStore()
	require.NoError(t, s.Seed())

	binary, ok := s.Get("trust-marketplace")
	require.True(t, ok)
	assert.Equal(t, 100, binary.Responses)
	assert.Equal(t, 55, binary.Counts[1])

	choice, ok := s.Get("optimize-for")
	require.True(t, ok)
	assert.False(t, choice.Binary())
	assert.Equal(t, 100, choice.Responses)
	assert.Equal(t, []int{50, 30, 20}, choice.Counts)
}

func TestStore_ConcurrentResponsesAreCounted(t *testing.T) {
	s := NewStore()
	poll, err := s.Create("q", 1.0, nil)
	require.NoError(t, err)

	const n = 200
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_, _ = s.RecordResponse(poll.ID, 1)
		}()
	}
	wg.Wait()

	got, ok := s.Get(poll.ID)
	require.True(t, ok)
	assert.Equal(t, n, got.Responses)
	assert.Equal(t, n, got.Counts[1])
}

package survey

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_CreateAndGet(t *testing.T) {
	s := NewStore()
	poll, err := s.Create("Do agents dream?", 1.0)
	require.NoError(t, err)
	assert.NotEmpty(t, poll.ID)

	got, ok := s.Get(poll.ID)
	require.True(t, ok)
	assert.Equal(t, poll.ID, got.ID)
	assert.Equal(t, "Do agents dream?", got.Question)
}

func TestStore_CreateRejectsBadInput(t *testing.T) {
	s := NewStore()
	_, err := s.Create("   ", 1.0)
	assert.Error(t, err)
	_, err = s.Create("ok", 0)
	assert.Error(t, err)
	_, err = s.Create("ok", -1)
	assert.Error(t, err)
}

func TestStore_RecordResponseTallies(t *testing.T) {
	s := NewStore()
	poll, err := s.Create("q", 1.0)
	require.NoError(t, err)

	_, err = s.RecordResponse(poll.ID, true)
	require.NoError(t, err)
	updated, err := s.RecordResponse(poll.ID, false)
	require.NoError(t, err)
	assert.Equal(t, 2, updated.Responses)
	assert.Equal(t, 1, updated.YesCount)
}

func TestStore_RecordResponseUnknownPoll(t *testing.T) {
	s := NewStore()
	_, err := s.RecordResponse("nope", true)
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

func TestStore_ConcurrentResponsesAreCounted(t *testing.T) {
	s := NewStore()
	poll, err := s.Create("q", 1.0)
	require.NoError(t, err)

	const n = 200
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_, _ = s.RecordResponse(poll.ID, true)
		}()
	}
	wg.Wait()

	got, ok := s.Get(poll.ID)
	require.True(t, ok)
	assert.Equal(t, n, got.Responses)
	assert.Equal(t, n, got.YesCount)
}

package auth

import (
	"context"
	"sync/atomic"
	"testing"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type stoppableTestSelector struct {
	stopCount atomic.Int32
}

func (s *stoppableTestSelector) Pick(context.Context, string, string, cliproxyexecutor.Options, []*Auth) (*Auth, error) {
	return nil, nil
}

func (s *stoppableTestSelector) Stop() {
	s.stopCount.Add(1)
}

func TestManagerSetSelector_StopsPreviousStoppableSelector(t *testing.T) {
	t.Parallel()

	previous := &stoppableTestSelector{}
	manager := NewManager(nil, previous, nil)

	manager.SetSelector(&RoundRobinSelector{})

	if got := previous.stopCount.Load(); got != 1 {
		t.Fatalf("previous selector stop count = %d, want %d", got, 1)
	}
}

func TestManagerStopAutoRefresh_StopsCurrentStoppableSelector(t *testing.T) {
	t.Parallel()

	current := &stoppableTestSelector{}
	manager := NewManager(nil, current, nil)

	manager.StopAutoRefresh()

	if got := current.stopCount.Load(); got != 1 {
		t.Fatalf("current selector stop count = %d, want %d", got, 1)
	}
}

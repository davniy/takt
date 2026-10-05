package runtime

import (
	"context"
	"sync"

	"takt/internal/assistant"
	"takt/internal/spec"
	"takt/internal/store"
)

type assistantLiveKey struct{}
type assistantBindingKey struct{}

type preparedAssistantBinding struct {
	resolved resolvedAssistantNode
	err      error
}

type assistantLiveWriter struct {
	mu     sync.Mutex
	runner *Runner
	state  *store.RunState
	err    error
	cancel context.CancelFunc
}

func (r *Runner) newAssistantLiveWriter(ctx context.Context, state *store.RunState) (context.Context, *assistantLiveWriter, error) {
	snapshot, err := cloneRunStateForPersistence(state)
	if err != nil {
		return ctx, nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	w := &assistantLiveWriter{runner: r, state: snapshot, cancel: cancel}
	return context.WithValue(ctx, assistantLiveKey{}, w), w, nil
}

func (w *assistantLiveWriter) emit(nodeID string, event assistant.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return
	}
	data := assistant.EventData(event)
	data["source"] = "adapter"
	w.err = w.runner.commit(w.state, "assistant."+event.Type, nodeID, data)
	if w.err != nil {
		w.cancel()
	}
}

func (w *assistantLiveWriter) finish(state *store.RunState) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cancel()
	state.Revision = w.state.Revision
	state.UpdatedAt = w.state.UpdatedAt
	state.HeartbeatAt = w.state.HeartbeatAt
	return w.err
}

func (r *Runner) prepareParallelAssistantBindings(state *store.RunState, nodes []spec.Node, previous map[string]store.NodeState) map[string]preparedAssistantBinding {
	bindings := map[string]preparedAssistantBinding{}
	for _, node := range nodes {
		if node.Command == "" && node.Prompt == "" {
			continue
		}
		action := r.actionContext(state, node, previous)
		resolved, err := r.resolveAssistantNode(state, node, action.local, action.feedback, action.artifacts)
		bindings[node.ID] = preparedAssistantBinding{resolved: resolved, err: err}
	}
	return bindings
}

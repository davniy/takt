package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"takt/internal/assistant"
	"takt/internal/spec"
	"takt/internal/store"
)

func TestAssistantLiveEvents(t *testing.T) {
	for _, mode := range []string{"single", "parallel", "loop"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			nodes := []spec.Node{{ID: "agent", Prompt: "review", Timeout: "5s"}}
			if mode == "parallel" {
				nodes = append(nodes, spec.Node{ID: "second", Prompt: "review", Timeout: "5s"})
			}
			cfg := &spec.Config{Models: map[string]spec.ModelSpec{"model": {Provider: "demo", ID: "demo"}}, Assistants: map[string]spec.AssistantSpec{"demo": {Type: "mock"}}}
			workflowNodes := nodes
			if mode == "loop" {
				workflowNodes = []spec.Node{{ID: "cycle", LoopGroup: &spec.LoopGroupSpec{MaxIterations: 1, Nodes: nodes, Until: spec.UntilSpec{Node: "agent", OutputContains: "done"}}}}
			}
			r := New(&spec.Workflow{Name: "live", Provider: "demo", Model: "model", Nodes: workflowNodes}, cfg, filepath.Join(dir, "workflow.yaml"), filepath.Join(dir, "config.yaml"), dir)
			emitted := make(chan string, len(nodes))
			release := make(chan struct{})
			r.assistants = resolverFunc(func(string) (assistant.Adapter, error) {
				return adapterFunc(func(ctx context.Context, req assistant.Request) (assistant.Result, error) {
					assistant.Emit(req, assistant.Event{Type: assistant.EventMessage, Message: "provider retry: 504 Gateway Time-out"})
					emitted <- req.RunID
					select {
					case <-release:
					case <-ctx.Done():
					}
					return assistant.Result{Output: "done"}, nil
				}), nil
			})
			done := make(chan error, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() { _, err := r.Start(ctx, ""); done <- err }()
			var runID string
			for range nodes {
				select {
				case id := <-emitted:
					runID = id
					events, err := (store.FS{Workspace: dir}).ReadEvents(id, 0, 0)
					if err != nil {
						t.Fatal(err)
					}
					found := false
					for _, event := range events {
						if event.Type == "assistant.message" && event.Data["message"] == "provider retry: 504 Gateway Time-out" {
							found = true
						}
					}
					if !found {
						t.Error("retry diagnostic not durable while adapter is running")
					}
					if _, err := (store.FS{Workspace: dir}).Load(id); err != nil {
						t.Fatal(err)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("adapter did not emit")
				}
			}
			close(release)
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("run did not stop")
			}
			events, err := (store.FS{Workspace: dir}).ReadEvents(runID, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			var revision uint64
			for _, event := range events {
				if event.Revision != revision+1 {
					t.Fatalf("non-contiguous revisions: %d after %d", event.Revision, revision)
				}
				revision = event.Revision
				if event.Data["message"] == "provider retry: 504 Gateway Time-out" {
					counts[event.NodeID]++
				}
			}
			for _, node := range nodes {
				if counts[node.ID] != 1 {
					t.Fatalf("event count for %s = %d", node.ID, counts[node.ID])
				}
			}
		})
	}
}

type failingLiveStore struct {
	store.Repository
	failure error
}

func (s failingLiveStore) Commit(state *store.RunState, event store.Event) error {
	if event.Type == "assistant.message" {
		return s.failure
	}
	return s.Repository.Commit(state, event)
}

func TestAssistantLiveEventsPersistenceFailure(t *testing.T) {
	dir := t.TempDir()
	cfg := &spec.Config{Models: map[string]spec.ModelSpec{"model": {Provider: "demo", ID: "demo"}}, Assistants: map[string]spec.AssistantSpec{"demo": {Type: "mock"}}}
	r := New(&spec.Workflow{Name: "live-failure", Provider: "demo", Model: "model", Nodes: []spec.Node{{ID: "agent", Prompt: "work", Timeout: "2s"}}}, cfg, filepath.Join(dir, "workflow.yaml"), filepath.Join(dir, "config.yaml"), dir)
	failure := errors.New("live persistence unavailable")
	r.store = failingLiveStore{Repository: r.store, failure: failure}
	r.assistants = resolverFunc(func(string) (assistant.Adapter, error) {
		return adapterFunc(func(ctx context.Context, req assistant.Request) (assistant.Result, error) {
			assistant.Emit(req, assistant.Event{Type: assistant.EventMessage, Message: "working"})
			<-ctx.Done()
			return assistant.Result{}, ctx.Err()
		}), nil
	})
	_, err := r.Start(context.Background(), "")
	if !errors.Is(err, failure) {
		t.Fatalf("lost persistence error: %v", err)
	}
}

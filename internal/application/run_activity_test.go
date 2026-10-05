package application

import (
	"encoding/json"
	"testing"

	"takt/internal/store"
)

func TestRunStatusShowsLiveAssistantObservation(t *testing.T) {
	s, fs, root, _ := observationFixture(t)
	root.Status = store.RunRunning
	root.Nodes["review"] = &store.NodeState{Status: store.NodeRunning}
	for _, event := range []store.Event{
		{Type: "node.started", NodeID: "review"},
		{Type: "assistant.session.started", NodeID: "review", Data: map[string]any{"idle_timeout": "5m", "timeout": "1h"}},
		{Type: "assistant.diagnostic", NodeID: "review", Data: map[string]any{"code": "pi.auto_retry.started", "message": "504 Gateway Time-out", "delay_ms": 2000}},
	} {
		if err := fs.Commit(root, event); err != nil {
			t.Fatal(err)
		}
	}
	status, err := s.RunService.Status(root.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(status)
	var result struct {
		Assistants []struct {
			NodeID         string `json:"node_id"`
			ObservedEvent  string `json:"observed_event"`
			DiagnosticCode string `json:"diagnostic_code"`
			IdleTimeout    string `json:"idle_timeout"`
			Timeout        string `json:"timeout"`
			LastEventAt    string `json:"last_event_at"`
		} `json:"assistants"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Assistants) != 1 {
		t.Fatalf("missing active assistant: %s", raw)
	}
	a := result.Assistants[0]
	if a.NodeID != "review" || a.DiagnosticCode != "pi.auto_retry.started" || a.ObservedEvent != "assistant.diagnostic" || a.IdleTimeout != "5m" || a.Timeout != "1h" || a.LastEventAt == "" {
		t.Fatalf("incomplete observation: %s", raw)
	}
	if err := fs.Commit(root, store.Event{Type: "assistant.tool.started", NodeID: "review", Data: map[string]any{"tool": "read"}}); err != nil {
		t.Fatal(err)
	}
	status, err = s.RunService.Status(root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Assistants) != 1 || status.Assistants[0].DiagnosticCode != "" || status.Assistants[0].Tool != "read" {
		t.Fatalf("stale retry: %+v", status.Assistants)
	}
	root.Nodes["review"].Status = store.NodeCompleted
	if err := fs.Commit(root, store.Event{Type: "node.completed", NodeID: "review"}); err != nil {
		t.Fatal(err)
	}
	status, err = s.RunService.Status(root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Assistants) != 0 {
		t.Fatalf("completed node shown as active: %+v", status.Assistants)
	}
}

package runtime

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"takt/internal/assistant"

	"takt/internal/command"
	"takt/internal/spec"
	"takt/internal/store"
)

func TestValidateCapabilitiesFailsBeforeRunCreation(t *testing.T) {
	allowed := []string{"read"}
	workflow := &spec.Workflow{Provider: "limited", Model: "demo", Nodes: []spec.Node{{ID: "agent", Prompt: "work", AllowedTools: &allowed}}}
	config := &spec.Config{Models: map[string]spec.ModelSpec{"demo": {Provider: "test", ID: "demo"}}, Assistants: map[string]spec.AssistantSpec{"limited": {Type: "process", Argv: []string{"cat"}}}}
	err := ValidateCapabilities(workflow, config, filepath.Join(t.TempDir(), "workflow.yaml"), command.Resolver{}, assistant.Factory{Config: config})
	if err == nil || !strings.Contains(err.Error(), "tool_policy") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateCapabilitiesChecksMatrixBody(t *testing.T) {
	allowed := []string{"read"}
	wf := &spec.Workflow{Provider: "limited", Model: "demo", Nodes: []spec.Node{{
		ID: "cases", Matrix: &spec.MatrixSpec{ItemsFrom: "$INPUTS.cases", As: "item", OutputNode: "agent", Nodes: []spec.Node{{
			ID: "agent", Prompt: "work", AllowedTools: &allowed,
		}}},
	}}}
	config := &spec.Config{Models: map[string]spec.ModelSpec{"demo": {Provider: "test", ID: "demo"}}, Assistants: map[string]spec.AssistantSpec{"limited": {Type: "process", Argv: []string{"cat"}}}}
	err := ValidateCapabilities(wf, config, filepath.Join(t.TempDir(), "workflow.yaml"), command.Resolver{}, assistant.Factory{Config: config})
	if err == nil || !strings.Contains(err.Error(), "tool_policy") {
		t.Fatalf("matrix capability error = %v", err)
	}
}

func TestValidateCapabilitiesRequiresTurnBudget(t *testing.T) {
	workflow := &spec.Workflow{Provider: "limited", Model: "demo", Nodes: []spec.Node{{ID: "agent", Prompt: "work", MaxTurns: 3}}}
	config := &spec.Config{
		Models:     map[string]spec.ModelSpec{"demo": {Provider: "test", ID: "demo"}},
		Assistants: map[string]spec.AssistantSpec{"limited": {Type: "process", Argv: []string{"cat"}}},
	}
	workflowPath := filepath.Join(t.TempDir(), "workflow.yaml")
	err := ValidateCapabilities(workflow, config, workflowPath, command.Resolver{}, assistant.Factory{Config: config})
	if err == nil || !strings.Contains(err.Error(), "turn_budget") {
		t.Fatalf("expected missing turn_budget capability for %s, got %v", workflowPath, err)
	}
}

func TestValidateCapabilitiesAcceptsDeclaredTurnBudget(t *testing.T) {
	workflow := &spec.Workflow{Provider: "limited", Model: "demo", Nodes: []spec.Node{{ID: "agent", Prompt: "work", MaxTurns: 3}}}
	config := &spec.Config{
		Models:     map[string]spec.ModelSpec{"demo": {Provider: "test", ID: "demo"}},
		Assistants: map[string]spec.AssistantSpec{"limited": {Type: "process", Argv: []string{"cat"}, Capabilities: []string{assistant.CapabilityTurnBudget}}},
	}
	workflowPath := filepath.Join(t.TempDir(), "workflow.yaml")
	if err := ValidateCapabilities(workflow, config, workflowPath, command.Resolver{}, assistant.Factory{Config: config}); err != nil {
		t.Fatalf("declared turn_budget rejected for %s: %v", workflowPath, err)
	}
}

func TestValidateCapabilitiesRejectsExternalMaxTurns(t *testing.T) {
	workflow := &spec.Workflow{Provider: "limited", Model: "demo", Nodes: []spec.Node{{ID: "agent", Prompt: "work", Executor: "external", MaxTurns: 3}}}
	config := &spec.Config{
		Models:     map[string]spec.ModelSpec{"demo": {Provider: "test", ID: "demo"}},
		Assistants: map[string]spec.AssistantSpec{"limited": {Type: "process", Argv: []string{"cat"}, Capabilities: []string{assistant.CapabilityTurnBudget}}},
	}
	workflowPath := filepath.Join(t.TempDir(), "workflow.yaml")
	err := ValidateCapabilities(workflow, config, workflowPath, command.Resolver{}, assistant.Factory{Config: config})
	if err == nil || !strings.Contains(err.Error(), "max_turns") {
		t.Fatalf("expected external max_turns rejection for %s, got %v", workflowPath, err)
	}
}

func TestAssistantResolveRejectsMaxTurnsWithoutTurnBudget(t *testing.T) {
	wf := &spec.Workflow{Name: "turn-budget", Provider: "demo", Model: "m", Nodes: []spec.Node{{ID: "agent", Prompt: "work", MaxTurns: 2}}}
	cfg := &spec.Config{Models: map[string]spec.ModelSpec{"m": {Provider: "demo", ID: "m"}}, Assistants: map[string]spec.AssistantSpec{"demo": {Type: "mock"}}}
	r := New(wf, cfg, "wf", "cfg", t.TempDir())
	state := &store.RunState{ID: "turn-budget", Nodes: map[string]*store.NodeState{"agent": {Status: store.NodeRunning, Attempts: 1}}, Approvals: map[string]string{}}
	_, err := r.executeAssistantAction(context.Background(), state, wf.Nodes[0], r.actionContext(state, wf.Nodes[0], nil))
	if err == nil || !strings.Contains(err.Error(), "turn_budget") {
		t.Fatalf("resolve without turn_budget = %v", err)
	}
}

func TestAssistantResolveRejectsExternalMaxTurns(t *testing.T) {
	wf := &spec.Workflow{Name: "external-turn-budget", Provider: "demo", Model: "m", Nodes: []spec.Node{{ID: "agent", Prompt: "work", Executor: "external", MaxTurns: 2}}}
	cfg := &spec.Config{Models: map[string]spec.ModelSpec{"m": {Provider: "demo", ID: "m"}}, Assistants: map[string]spec.AssistantSpec{"demo": {Type: "mock"}}}
	r := New(wf, cfg, "wf", "cfg", t.TempDir())
	state := &store.RunState{ID: "external-turn-budget", Nodes: map[string]*store.NodeState{"agent": {Status: store.NodeRunning, Attempts: 1}}, Approvals: map[string]string{}}
	_, err := r.executeAssistantAction(context.Background(), state, wf.Nodes[0], r.actionContext(state, wf.Nodes[0], nil))
	if err == nil || !strings.Contains(err.Error(), "max_turns") {
		t.Fatalf("external resolve with max_turns = %v", err)
	}
}

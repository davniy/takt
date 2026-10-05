package pi

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	core "takt/internal/assistant"
)

func TestPiMalformedPathAllowsCorrectedToolCall(t *testing.T) {
	for _, args := range []string{`{}`, `null`, `{"path":null}`, `{"path":42}`, `{"path":"  "}`, `[]`} {
		t.Run(args, func(t *testing.T) {
			records := make(chan piRPCRecord, 5)
			records <- piRPCRecord{Type: "tool_execution_start", Raw: json.RawMessage(`{"toolCallId":"bad","toolName":"edit","args":` + args + `}`)}
			records <- piRPCRecord{Type: "tool_execution_end", Raw: json.RawMessage(`{"toolCallId":"bad","toolName":"edit","isError":true}`)}
			records <- piRPCRecord{Type: "tool_execution_start", Raw: json.RawMessage(`{"toolCallId":"fixed","toolName":"edit","args":{"path":"route.yaml"}}`)}
			records <- piRPCRecord{Type: "tool_execution_end", Raw: json.RawMessage(`{"toolCallId":"fixed","toolName":"edit","isError":false}`)}
			records <- piRPCRecord{Type: "agent_settled"}
			workspace := t.TempDir()
			var events []core.Event
			client := piRPCClient{
				records: records,
				process: &piProcessWait{done: make(chan struct{})},
				request: Request{Emit: func(event core.Event) {
					if err := core.ValidateEvent(event); err != nil {
						t.Fatal(err)
					}
					if event.Type == core.EventToolStarted || event.Type == core.EventToolRequested {
						if err := core.ValidateToolPath(event.Tool, event.Input, workspace, ""); err != nil {
							t.Fatalf("recoverable Pi arguments caused collector failure: %v", err)
						}
					}
					events = append(events, event)
				}},
			}
			if _, err := client.waitAgentSettled(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(events) != 4 || events[0].Type != core.EventDiagnostic || events[0].CallID != "bad" || events[0].Data["code"] != "pi.tool.invalid_path_arguments" || events[1].Data["error"] != true || events[2].Type != core.EventToolStarted || events[3].Data["error"] != false {
				t.Fatalf("lost error/correction lifecycle: %+v", events)
			}
		})
	}
}

func TestPiNonemptyMutationPathStillReachesCollector(t *testing.T) {
	for _, tool := range []string{"edit", "write", "patch"} {
		event, ok := piProgressEvent(piRPCRecord{Type: "tool_execution_start", Raw: json.RawMessage(`{"toolCallId":"outside","toolName":"` + tool + `","args":{"path":"/outside/route.yaml"}}`)})
		if !ok || event.Type != core.EventToolStarted {
			t.Fatalf("path violation hidden: %+v", event)
		}
		if err := core.ValidateToolPath(event.Tool, event.Input, t.TempDir(), ""); err == nil {
			t.Fatal("outside path accepted")
		}
	}
}

func TestPiOutsideMutationPathIsPrevalidationDiagnostic(t *testing.T) {
	workspace := t.TempDir()
	records := make(chan piRPCRecord, 5)
	records <- piRPCRecord{Type: "tool_execution_start", Raw: json.RawMessage(`{"toolCallId":"outside","toolName":"write","args":{"path":"/wrong/workspace/route.yaml"}}`)}
	records <- piRPCRecord{Type: "tool_execution_end", Raw: json.RawMessage(`{"toolCallId":"outside","toolName":"write","isError":true}`)}
	records <- piRPCRecord{Type: "tool_execution_start", Raw: json.RawMessage(`{"toolCallId":"fixed","toolName":"write","args":{"path":"route.yaml"}}`)}
	records <- piRPCRecord{Type: "tool_execution_end", Raw: json.RawMessage(`{"toolCallId":"fixed","toolName":"write","isError":false}`)}
	records <- piRPCRecord{Type: "agent_settled"}
	var events []core.Event
	client := piRPCClient{
		records: records,
		process: &piProcessWait{done: make(chan struct{})},
		request: Request{Workspace: workspace, Emit: func(event core.Event) {
			if err := core.ValidateEvent(event); err != nil {
				t.Fatal(err)
			}
			if event.Type == core.EventToolStarted || event.Type == core.EventToolRequested {
				if err := core.ValidateToolPath(event.Tool, event.Input, workspace, ""); err != nil {
					t.Fatalf("prevalidated path reached collector as execution: %v", err)
				}
			}
			events = append(events, event)
		}},
	}
	if _, err := client.waitAgentSettled(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("unexpected event count: %+v", events)
	}
	if events[0].Type != core.EventDiagnostic || events[0].Data["code"] != "pi.tool.prevalidation_denied" {
		t.Fatalf("outside mutation was treated as execution: %+v", events[0])
	}
	if events[1].Type != core.EventToolCompleted || events[1].Data["error"] != true {
		t.Fatalf("denied completion was lost: %+v", events[1])
	}
	if events[2].Type != core.EventToolStarted || events[3].Type != core.EventToolCompleted || events[3].Data["error"] != false {
		t.Fatalf("corrected mutation lifecycle was lost: %+v", events)
	}
}

func TestPiNativeGuardRejectsMalformedPathAndAllowsCorrection(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for native Pi guard test")
	}
	guard, cleanup, err := installWorkspaceGuard()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	script := filepath.Join(t.TempDir(), "check.mjs")
	if err := os.WriteFile(script, []byte(`import guard from "`+filepath.ToSlash(guard)+`"
let hook
guard({on(name, fn) { if (name === "tool_call") hook = fn }, registerTool() {}})
for (const toolName of ["write", "edit", "patch"]) {
  for (const input of [{}, null, {path:null}, {path:42}, {path:"  "}, []]) {
    const result = await hook({toolName, input})
    if (!result?.block || !result.reason) throw new Error("malformed path allowed")
  }
  if ((await hook({toolName, input:{path:"route.yaml"}}))?.block) throw new Error("correction blocked")
}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, script)
	cmd.Env = append(os.Environ(), "TAKT_WORKSPACE="+t.TempDir(), "TAKT_ARTIFACTS_DIR=")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native guard recovery: %v\n%s", err, output)
	}
}

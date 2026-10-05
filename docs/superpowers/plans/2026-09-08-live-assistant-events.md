# Live assistant events implementation plan

> Use `superpowers:executing-plans`; implementation in the current session without subagents or commits, as requested by the user.

**Goal:** Make ordinary Run assistant progress observable before an adapter returns, while retaining bounded inactivity and cancellation semantics.

**Architecture:** Use the existing Store.Commit and event schema. A serialized writer owns a persistence snapshot for an active assistant action or parallel wave; action workers do not mutate the shared persistence revision. Apply terminal node results in existing scheduler order. Do not introduce a provider timeout or a second executor.

**Tech stack:** Go, existing runtime/Store, deterministic adapter contracts.

## Agreed design and evidence

The assignment-repair experiment has a one-hour node timeout and no idle_timeout. Pi session records show repeated 504 responses while the ordinary Run journal stops at node.started. executeAssistantAction exposes a live observer, but executeAttempt/runProviderExecution/runParallelWave flush normalized events only after completion. Existing idle monitoring should be reused, not replaced.

## Tasks

- [ ] Add `internal/runtime/assistant_live_events_test.go`: adapter emits a retry diagnostic and waits on a release channel; assert Store.ReadEvents and Store.Load expose it while the node is running, with no duplicate after completion. Cover root, loop and parallel paths.
- [ ] Run `go test ./internal/runtime -run TestAssistantLiveEvents -count=1`; confirm missing live persistence.
- [ ] Add a serialized snapshot writer in `internal/runtime/assistant_live_events.go`. Wire collector persistence, prepare parallel assistant bindings before concurrent execution, preserve original persistence errors and synchronize revision before scheduler transitions.
- [ ] Repeat the regression and run focused runtime/Pi tests with race detection. Add persistence-failure and idle/cancel cases as needed; do not replace timeout or settlement behavior.
- [ ] Expose observed assistant phase and last-event time in ordinary Run status using persisted events. Report unknown activity explicitly; events do not prove provider-side execution.
- [ ] Set explicit five-minute idle timeouts in the source assignment flow in micro-spec-coder; do not modify or restart the active experiment.
- [ ] Update runtime contract, implementation status, changelog and decision note. Implementation checks do not belong in the route experiment log.

## Acceptance

Live diagnostics survive interruption; event revisions remain monotonic and unique; secrets are redacted; failures of persistence are returned; timeout/cancellation retain precedence over derived adapter failures; Pi owns internal retries and settlement. Runtime-only evidence does not establish route-generation quality or resolve external 504 failures.

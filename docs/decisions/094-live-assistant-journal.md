# Live assistant journal

Ordinary Runs previously buffered normalized assistant events until action
completion. This hid Pi provider retries behind node.started even though adapter
observers received them live. The assignment experiment had timeout=1h without
idle_timeout; this is not evidence that existing timeout cancellation is broken.

Persist normalized events through the existing Store.Commit during execution.
An action owns an isolated persistence snapshot. A parallel wave prepares its
assistant bindings sequentially and shares one synchronized snapshot writer.
Only revision/update/heartbeat fields return to scheduler state before terminal
results are applied in their existing order. Failed persistence cancels the
writer's context and returns the original error. No second scheduler, journal,
provider retry owner or timeout field is introduced.

Status reports observations, not guessed model/network states. Transient activity
remains non-durable and last-event time must not be labelled last activity.
Five-minute idle limits belong to the coder assignment workflow, not a global
runtime default. Existing running experiments are not rewritten or restarted.

The cost is synchronous Store.Commit per normalized event. Token deltas remain
excluded, but long runs with many normalized events may require future measured
Store optimization. Batching, recovery changes and default timeout policies are
outside this change.

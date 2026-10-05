# ADR-093. Pi pre-validation events do not prove execution

Status: accepted, implementation under functional evaluation.

The installed Pi agent loop emits `tool_execution_start` before preparing and
validating arguments, including before native `beforeToolCall` interception.
Treating every such event as a normalized mutation start cancelled a real Route
DSL writer when its `edit` call omitted `path`. Pi never had the opportunity to
return the native tool error to the model.

Normalize missing/non-string/blank mutation paths as bounded diagnostic
observations with original arguments and call ID. Do not claim that the tool
has executed or that a deny decision has already occurred. Native argument
validation and the mandatory workspace guard remain responsible for rejecting
the call. Preserve the completion error flag and subsequent corrected calls.

Keep nonempty-path collector enforcement and the blocking tool-control
contract unchanged. The collector now tolerates malformed-path lifecycle
events generically (dropping them instead of cancelling the attempt) for any
adapter, while concrete outside-workspace paths remain fail-closed. This is a
Pi adapter correction, not a second tool loop, blanket protocol retry, or
relaxed workspace boundary. A passing assistant attempt still does not prove
route correctness; downstream functional checks remain authoritative.

Regression: replay the malformed start, error completion, corrected start and
successful completion through the Pi RPC client. Exercise the installed native
guard with malformed paths and a valid correction; retain concrete external
path and symlink escape tests. The original regression failed before the fix.

Live functional continuation uses the existing failed Route DSL Run via native
operator retry. The current iteration recovered the retained Pi Session ID and
appended further tool calls to the same session file. This is not a fresh
end-to-end one-shot evaluation.

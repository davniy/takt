package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"takt/internal/application"
)

func formatTextResult(value any) (string, bool) {
	switch v := value.(type) {
	case *application.RunStatusResult:
		if v == nil {
			return "", false
		}
		return formatRunStatusText(*v), true
	case application.RunStatusResult:
		return formatRunStatusText(v), true
	case *application.RunStatsResult:
		if v == nil {
			return "", false
		}
		return formatRunStatsText(*v), true
	case application.RunStatsResult:
		return formatRunStatsText(v), true
	case *application.RunInspectResult:
		if v == nil {
			return "", false
		}
		return formatRunInspectText(*v), true
	case application.RunInspectResult:
		return formatRunInspectText(v), true
	case *application.RunState:
		if v == nil {
			return "", false
		}
		return formatRunStateText(*v), true
	case application.RunState:
		return formatRunStateText(v), true
	default:
		return "", false
	}
}

func formatRunStatusText(status application.RunStatusResult) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)

	fmt.Fprintln(w, "RUN")
	fmt.Fprintf(w, "  ID\t%s\n", status.RunID)
	fmt.Fprintf(w, "  Status\t%s\n", status.Status)
	if status.ErrorCode != "" {
		fmt.Fprintf(w, "  Error code\t%s\n", status.ErrorCode)
	}
	if status.Error != "" {
		fmt.Fprintf(w, "  Error\t%s\n", status.Error)
	}
	if status.CurrentNode != "" {
		fmt.Fprintf(w, "  Current\t%s\n", status.CurrentNode)
	}
	fmt.Fprintf(w, "  Attempts\t%d\n", status.Attempts)
	fmt.Fprintf(w, "  Executions\t%d\n", status.Executions)

	if status.Matrix.Total > 0 {
		fmt.Fprintln(w, "\nMATRIX")
		fmt.Fprintf(w, "  Total\t%d\n", status.Matrix.Total)
		fmt.Fprintf(w, "  Completed\t%d\n", status.Matrix.Completed)
		fmt.Fprintf(w, "  Failed\t%d\n", status.Matrix.Failed)
		fmt.Fprintf(w, "  Running\t%d\n", status.Matrix.Running)
		fmt.Fprintf(w, "  Pending\t%d\n", status.Matrix.Pending)
	}

	if status.Assessment.Primary > 0 || status.Assessment.Advisory > 0 || len(status.Assessment.Outcomes) > 0 {
		fmt.Fprintln(w, "\nASSESSMENT")
		fmt.Fprintf(w, "  Primary\t%d\n", status.Assessment.Primary)
		fmt.Fprintf(w, "  Advisory\t%d\n", status.Assessment.Advisory)
		fmt.Fprintf(w, "  Outcomes\t%s\n", formatCounts(status.Assessment.Outcomes))
	}

	if status.Usage != nil {
		fmt.Fprintln(w, "\nUSAGE")
		fmt.Fprintf(w, "  Input tokens\t%s\n", formatNumber(int64(status.Usage.InputTokens)))
		fmt.Fprintf(w, "  Output tokens\t%s\n", formatNumber(int64(status.Usage.OutputTokens)))
		fmt.Fprintf(w, "  Total tokens\t%s\n", formatNumber(int64(status.Usage.InputTokens+status.Usage.OutputTokens)))
	}

	if len(status.Assistants) > 0 {
		fmt.Fprintln(w, "\nACTIVE ASSISTANTS")
		fmt.Fprintln(w, "  Node\tEvent\tIdle timeout\tTimeout\tMessage")
		for _, a := range status.Assistants {
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", a.NodeID, a.ObservedEvent, valueOrDash(a.IdleTimeout), valueOrDash(a.Timeout), valueOrDash(a.Message))
		}
	}

	_ = w.Flush()
	return strings.TrimSpace(b.String())
}

func formatRunStatsText(stats application.RunStatsResult) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)

	gatesResult := "PASSED"
	if !stats.GatesPassed {
		gatesResult = "FAILED"
	}

	fmt.Fprintln(w, "EVALUATION STATS")
	fmt.Fprintf(w, "  Run ID\t%s\n", stats.RunID)
	fmt.Fprintf(w, "  Status\t%s\n", stats.Status)
	fmt.Fprintf(w, "  Gates\t%s\n", gatesResult)

	fmt.Fprintln(w, "\nMETRICS")
	fmt.Fprintf(w, "  Cases total\t%d\n", stats.Total)
	fmt.Fprintf(w, "  Cases evaluated\t%d\n", stats.Evaluated)
	fmt.Fprintf(w, "  Outcomes\t%s\n", formatCounts(stats.Outcomes))
	fmt.Fprintf(w, "  Valid rate\t%s\n", formatRatio(stats.ValidRate))
	fmt.Fprintf(w, "  Flow completion rate\t%s\n", formatRatio(stats.FlowCompletionRate))
	fmt.Fprintf(w, "  False accept rate\t%s\n", formatRatio(stats.FalseAcceptRate))
	fmt.Fprintf(w, "  False reject rate\t%s\n", formatRatio(stats.FalseRejectRate))
	fmt.Fprintf(w, "  Validation error rate\t%s\n", formatRatio(stats.ValidationErrorRate))

	if stats.Attempts > 0 || stats.Executions > 0 || stats.Usage != nil {
		fmt.Fprintln(w, "\nRESOURCES")
		fmt.Fprintf(w, "  Node attempts\t%d\n", stats.Attempts)
		fmt.Fprintf(w, "  Assistant executions\t%d\n", stats.Executions)
		if stats.Usage != nil {
			fmt.Fprintf(w, "  Input tokens\t%s\n", formatNumber(int64(stats.Usage.InputTokens)))
			fmt.Fprintf(w, "  Output tokens\t%s\n", formatNumber(int64(stats.Usage.OutputTokens)))
			fmt.Fprintf(w, "  Total tokens\t%s\n", formatNumber(int64(stats.Usage.InputTokens+stats.Usage.OutputTokens)))
		}
	}

	if len(stats.Gates) > 0 {
		fmt.Fprintln(w, "\nGATES")
		fmt.Fprintln(w, "  Metric\tThreshold\tActual\tResult")
		for _, g := range stats.Gates {
			thresh := "-"
			if g.Minimum != nil {
				thresh = fmt.Sprintf("min=%.3f", *g.Minimum)
			} else if g.Maximum != nil {
				thresh = fmt.Sprintf("max=%.3f", *g.Maximum)
			}
			actual := "-"
			if g.Actual != nil {
				actual = fmt.Sprintf("%.3f", *g.Actual)
			}
			res := "PASSED"
			if !g.Passed {
				res = "FAILED"
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", g.Metric, thresh, actual, res)
		}
	}

	_ = w.Flush()
	return strings.TrimSpace(b.String())
}

func formatRunInspectText(inspect application.RunInspectResult) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)

	fmt.Fprintln(w, "RUN INSPECTION")
	fmt.Fprintf(w, "  Run ID\t%s\n", inspect.RunID)
	fmt.Fprintf(w, "  Status\t%s\n", inspect.Status)
	fmt.Fprintf(w, "  Attempts\t%d\n", inspect.Attempts)
	fmt.Fprintf(w, "  Executions\t%d\n", inspect.Executions)
	if inspect.Cause.Code != "" || inspect.Cause.Source != "" {
		cause := inspect.Cause.Code
		if inspect.Cause.Source != "" {
			cause = fmt.Sprintf("%s (%s)", cause, inspect.Cause.Source)
		}
		if inspect.Cause.Message != "" {
			cause += ": " + inspect.Cause.Message
		}
		fmt.Fprintf(w, "  Cause\t%s\n", cause)
	}

	if len(inspect.Cases) > 0 {
		fmt.Fprintf(w, "\nCASES (%d)\n", len(inspect.Cases))
		fmt.Fprintln(w, "  Case\tRepeat\tStatus\tValid\tOutcome\tTarget Run\tCause")
		for _, c := range inspect.Cases {
			validStr := "-"
			if c.Valid != nil {
				if *c.Valid {
					validStr = "yes"
				} else {
					validStr = "no"
				}
			}
			cause := "-"
			if c.Cause.Code != "" || c.Cause.Source != "" {
				cause = c.Cause.Code
				if c.Cause.Source != "" {
					cause = fmt.Sprintf("%s (%s)", cause, c.Cause.Source)
				}
				if c.Cause.Message != "" {
					cause += ": " + c.Cause.Message
				}
			}
			fmt.Fprintf(w, "  %s\t%d\t%s\t%s\t%s\t%s\t%s\n",
				c.CaseID, c.Repeat, valueOrDash(c.TargetStatus), validStr, valueOrDash(c.Outcome), valueOrDash(c.TargetRunID), cause)
		}
	}

	if len(inspect.Nodes) > 0 {
		fmt.Fprintf(w, "\nNODES (%d)\n", len(inspect.Nodes))
		fmt.Fprintln(w, "  Node\tRun\tStatus\tAttempts\tCause")
		for _, n := range inspect.Nodes {
			cause := "-"
			if n.ErrorCode != "" || n.Error != "" {
				cause = joinCause(n.ErrorCode, n.Error)
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%d\t%s\n", n.NodeID, n.RunID, n.Status, n.Attempts, cause)
		}
	}

	_ = w.Flush()
	return strings.TrimSpace(b.String())
}

func formatRunStateText(state application.RunState) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)

	fmt.Fprintln(w, "RUN")
	fmt.Fprintf(w, "  ID\t%s\n", state.ID)
	fmt.Fprintf(w, "  Status\t%s\n", state.Status)
	if state.ErrorCode != "" {
		fmt.Fprintf(w, "  Error code\t%s\n", state.ErrorCode)
	}
	if state.Error != "" {
		fmt.Fprintf(w, "  Error\t%s\n", state.Error)
	}
	if state.CurrentNode != "" {
		fmt.Fprintf(w, "  Current\t%s\n", state.CurrentNode)
	}
	if state.WorkflowPath != "" {
		fmt.Fprintf(w, "  Workflow\t%s\n", state.WorkflowPath)
	}
	if state.ExecutionWorkspace != "" {
		fmt.Fprintf(w, "  Workspace\t%s\n", state.ExecutionWorkspace)
	} else if state.Workspace != "" {
		fmt.Fprintf(w, "  Workspace\t%s\n", state.Workspace)
	}

	if len(state.Nodes) > 0 {
		fmt.Fprintf(w, "\nNODES (%d)\n", len(state.Nodes))
		fmt.Fprintln(w, "  Node\tStatus\tAttempts\tError")
		nodeIDs := make([]string, 0, len(state.Nodes))
		for id := range state.Nodes {
			nodeIDs = append(nodeIDs, id)
		}
		sort.Strings(nodeIDs)
		for _, id := range nodeIDs {
			node := state.Nodes[id]
			cause := "-"
			if node != nil && (node.ErrorCode != "" || node.Error != "") {
				cause = joinCause(node.ErrorCode, node.Error)
			}
			attempts := 0
			status := "-"
			if node != nil {
				attempts = node.Attempts
				status = node.Status
			}
			fmt.Fprintf(w, "  %s\t%s\t%d\t%s\n", id, status, attempts, cause)
		}
	}

	_ = w.Flush()
	return strings.TrimSpace(b.String())
}

func formatRatio(r application.MetricRatio) string {
	if r.Value == nil {
		if r.Denominator == 0 {
			return "- (0/0)"
		}
		return fmt.Sprintf("- (%d/%d)", r.Numerator, r.Denominator)
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", *r.Value*100, r.Numerator, r.Denominator)
}

func formatCounts(values map[string]int) string {
	if len(values) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, values[k]))
	}
	return strings.Join(parts, ", ")
}

func valueOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func joinCause(code, message string) string {
	if code == "" {
		return message
	}
	if message == "" {
		return code
	}
	return code + ": " + message
}

func formatNumber(value int64) string {
	raw := strconv.FormatInt(value, 10)
	start := 0
	if strings.HasPrefix(raw, "-") {
		start = 1
	}
	for index := len(raw) - 3; index > start; index -= 3 {
		raw = raw[:index] + " " + raw[index:]
	}
	return raw
}

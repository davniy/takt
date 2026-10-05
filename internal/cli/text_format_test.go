package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	"takt/internal/application"
	"takt/internal/assessment"
	"takt/internal/store"
)

func TestFormatRunStatusText(t *testing.T) {
	status := &application.RunStatusResult{
		RunID:      "run-ab0207579f6bb5e3cdf02054",
		Status:     "completed",
		Attempts:   16,
		Executions: 16,
		Matrix: application.RunMatrixProgress{
			Total:     1,
			Completed: 1,
			Failed:    0,
			Running:   0,
			Pending:   0,
		},
		Usage: &store.Usage{
			InputTokens:  2501153,
			OutputTokens: 105121,
		},
		Assessment: application.RunAssessmentSummary{
			Primary:  1,
			Advisory: 0,
			Outcomes: map[string]int{
				"true_accept": 1,
			},
		},
	}

	text, ok := formatTextResult(status)
	if !ok {
		t.Fatal("expected formatTextResult to handle *application.RunStatusResult")
	}

	for _, want := range []string{
		"RUN",
		"run-ab0207579f6bb5e3cdf02054",
		"completed",
		"16",
		"MATRIX",
		"Total",
		"Completed",
		"ASSESSMENT",
		"true_accept=1",
		"USAGE",
		"2 501 153",
		"105 121",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

func TestFormatRunStatsText(t *testing.T) {
	val1 := 1.0
	val0 := 0.0
	stats := &application.RunStatsResult{
		RunID:               "run-ab0207579f6bb5e3cdf02054",
		Status:              "completed",
		Total:               1,
		Evaluated:           1,
		Attempts:            16,
		Executions:          16,
		Outcomes:            map[string]int{"true_accept": 1},
		ValidRate:           application.MetricRatio{Numerator: 1, Denominator: 1, Value: &val1},
		FlowCompletionRate:  application.MetricRatio{Numerator: 1, Denominator: 1, Value: &val1},
		FalseAcceptRate:     application.MetricRatio{Numerator: 0, Denominator: 1, Value: &val0},
		FalseRejectRate:     application.MetricRatio{Numerator: 0, Denominator: 1, Value: &val0},
		ValidationErrorRate: application.MetricRatio{Numerator: 0, Denominator: 1, Value: &val0},
		GatesPassed:         true,
		Usage: &store.Usage{
			InputTokens:  2501153,
			OutputTokens: 105121,
		},
		Gates: []application.AssessmentGateResult{
			{
				Metric:      "valid_rate",
				Passed:      true,
				Numerator:   1,
				Denominator: 1,
				Actual:      &val1,
				Minimum:     &val1,
				Message:     "valid_rate 1.000 >= min 1.000",
			},
		},
	}

	text, ok := formatTextResult(stats)
	if !ok {
		t.Fatal("expected formatTextResult to handle *application.RunStatsResult")
	}

	for _, want := range []string{
		"EVALUATION STATS",
		"run-ab0207579f6bb5e3cdf02054",
		"completed",
		"PASSED",
		"METRICS",
		"Cases total",
		"Cases evaluated",
		"true_accept=1",
		"100.0% (1/1)",
		"0.0% (0/1)",
		"GATES",
		"valid_rate",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

func TestFormatRunInspectText(t *testing.T) {
	valid := true
	inspection := &application.RunInspectResult{
		RunID:      "run-ab0207579f6bb5e3cdf02054",
		Status:     "completed",
		Attempts:   16,
		Executions: 16,
		Cause: application.RunCause{
			Source: "run",
			Code:   "completed",
		},
		Cases: []application.RunCaseInspection{
			{
				CaseID:       "implement-basic",
				Repeat:       1,
				TargetRunID:  "20260914T111157-adcbf18cd813dc50",
				TargetStatus: "completed",
				Outcome:      "true_accept",
				Valid:        &valid,
				Cause: application.RunCause{
					Source: "assessment",
					Code:   "true_accept",
				},
				Evidence: []assessment.EvidenceRef{
					{
						ProducerRunID: "run-ab0207579f6bb5e3cdf02054",
						ArtifactID:    "cases__evidence:evaluation-evidence:matrix-0000:1",
						SHA256:        "ca2a7e5160835a113afb577e282e2f7f1ce2bc14903b7d156473a38f5e8fa738",
					},
				},
			},
		},
		Nodes: []application.RunNodeInspection{
			{
				RunID:      "20260914T111157-adcbf18cd813dc50",
				NodeID:     "implement",
				Status:     "completed",
				Attempts:   1,
				Executions: 1,
			},
		},
	}

	text, ok := formatTextResult(inspection)
	if !ok {
		t.Fatal("expected formatTextResult to handle *application.RunInspectResult")
	}

	for _, want := range []string{
		"RUN INSPECTION",
		"run-ab0207579f6bb5e3cdf02054",
		"completed",
		"CASES (1)",
		"implement-basic",
		"true_accept",
		"NODES (1)",
		"implement",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

func TestPrintResultRendersFormattedTextWhenNotJSON(t *testing.T) {
	status := &application.RunStatusResult{
		RunID:      "run-123",
		Status:     "completed",
		Attempts:   1,
		Executions: 1,
	}

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	if err := printResult(false, status); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	output := string(raw)

	if strings.HasPrefix(output, "&{RunID:") {
		t.Fatalf("printResult printed raw Go struct instead of formatted text:\n%s", output)
	}
	if !strings.Contains(output, "RUN") || !strings.Contains(output, "run-123") {
		t.Fatalf("printResult output missing expected fields:\n%s", output)
	}
}

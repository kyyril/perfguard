package cli

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNightlyEvalWorkflowYML(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/nightly-eval.yml")
	if err != nil {
		t.Fatalf("read nightly-eval.yml: %v", err)
	}

	var wf struct {
		Name        string `yaml:"name"`
		On          map[string]interface{} `yaml:"on"`
		Permissions map[string]string `yaml:"permissions"`
		Jobs        map[string]struct{
			Name    string `yaml:"name"`
			RunsOn  string `yaml:"runs-on"`
			Timeout string `yaml:"timeout-minutes"`
			Steps   []struct{
				Name string `yaml:"name"`
				Uses string `yaml:"uses,omitempty"`
				Run  string `yaml:"run,omitempty"`
				With map[string]interface{} `yaml:"with,omitempty"`
				If   string `yaml:"if,omitempty"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}

	if err := yaml.Unmarshal(data, &wf); err != nil {
		t.Fatalf("unmarshal workflow: %v", err)
	}

	// Check workflow metadata
	if wf.Name != "Nightly Evaluation" {
		t.Errorf("name = %q, want %q", wf.Name, "Nightly Evaluation")
	}

	// Check triggers
	if wf.On == nil {
		t.Fatal("on triggers not defined")
	}
	if _, hasSchedule := wf.On["schedule"]; !hasSchedule {
		t.Error("missing schedule trigger")
	}
	if _, hasDispatch := wf.On["workflow_dispatch"]; !hasDispatch {
		t.Error("missing workflow_dispatch trigger")
	}

	// Check permissions
	if wf.Permissions["contents"] != "read" {
		t.Errorf("contents permission = %q, want %q", wf.Permissions["contents"], "read")
	}

	// Check eval job
	evalJob, ok := wf.Jobs["eval"]
	if !ok {
		t.Fatal("eval job not found")
	}

	if evalJob.Name != "Run Evaluation Suite" {
		t.Errorf("job name = %q, want %q", evalJob.Name, "Run Evaluation Suite")
	}

	if evalJob.RunsOn != "ubuntu-latest" {
		t.Errorf("runs-on = %q, want %q", evalJob.RunsOn, "ubuntu-latest")
	}

	// Check required steps
	requiredSteps := map[string]bool{
		"Checkout code":       false,
		"Set up Go":           false,
		"Build PerfGuard":     false,
		"Run all scenarios":   false,
		"Generate report":     false,
		"Upload results":      false,
		"Upload report":       false,
	}

	for _, step := range evalJob.Steps {
		if _, exists := requiredSteps[step.Name]; exists {
			requiredSteps[step.Name] = true
		}
	}

	for stepName, found := range requiredSteps {
		if !found {
			t.Errorf("missing required step: %q", stepName)
		}
	}

	// Verify artifact upload steps have if: always()
	for _, step := range evalJob.Steps {
		if step.Uses != "" && step.Uses == "actions/upload-artifact@v4" {
			if step.If != "always()" {
				t.Errorf("upload artifact step %q missing if: always()", step.Name)
			}
		}
	}

	// Verify run scenarios step loops over eval/scenarios/*
	var foundScenarioLoop bool
	for _, step := range evalJob.Steps {
		if step.Name == "Run all scenarios" && step.Run != "" {
			if contains(step.Run, "eval/scenarios/*") && contains(step.Run, "perfguard eval run") {
				foundScenarioLoop = true
			}
		}
	}
	if !foundScenarioLoop {
		t.Error("scenario loop not found or malformed")
	}

	// Verify report generation step
	var foundReportGen bool
	for _, step := range evalJob.Steps {
		if step.Name == "Generate report" && step.Run != "" {
			if contains(step.Run, "perfguard eval report") && contains(step.Run, "eval/RESULTS.md") {
				foundReportGen = true
			}
		}
	}
	if !foundReportGen {
		t.Error("report generation step not found or malformed")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

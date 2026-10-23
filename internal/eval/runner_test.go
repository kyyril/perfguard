package eval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyyril/perfguard/internal/model"
)

type fakeWorkspace struct {
	dir    string
	closed bool
}

func (w *fakeWorkspace) Dir() string { return w.dir }

func (w *fakeWorkspace) Close(_ context.Context) error {
	w.closed = true
	return nil
}

type fakePreparer struct {
	workspace *fakeWorkspace
	err       error
	calls     int
}

func (p *fakePreparer) Prepare(_ context.Context, _, _ string) (Workspace, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return p.workspace, nil
}

type fakeChecker struct {
	comparison model.Comparison
	err        error
	calls      int
}

func (c *fakeChecker) RunCheck(_ context.Context, _ CheckRequest) (model.Comparison, error) {
	c.calls++
	if c.err != nil {
		return model.Comparison{}, c.err
	}
	return c.comparison, nil
}

func TestRun_SingleTrial(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := createTestScenario(t, tmpDir, "single-trial", 1)
	outputDir := filepath.Join(tmpDir, "output")

	fakePreparer := &fakePreparer{
		workspace: &fakeWorkspace{dir: tmpDir},
	}
	fakeChecker := &fakeChecker{
		comparison: model.Comparison{
			SchemaVersion: model.CurrentSchemaVersion,
			Verdict:       model.VerdictPass,
		},
	}

	result, err := Run(context.Background(), RunOptions{
		ScenarioDir: scenarioDir,
		OutputDir:   outputDir,
		Preparer:    fakePreparer,
		Checker:     fakeChecker,
	})

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result.ScenarioID != "single-trial" {
		t.Errorf("ScenarioID = %q, want %q", result.ScenarioID, "single-trial")
	}

	if len(result.Trials) != 1 {
		t.Fatalf("len(Trials) = %d, want 1", len(result.Trials))
	}

	if result.Trials[0].Trial != 1 {
		t.Errorf("Trial[0].Trial = %d, want 1", result.Trials[0].Trial)
	}

	if result.Trials[0].Verdict != model.VerdictPass {
		t.Errorf("Trial[0].Verdict = %q, want %q", result.Trials[0].Verdict, model.VerdictPass)
	}

	if result.Trials[0].Error != "" {
		t.Errorf("Trial[0].Error = %q, want empty", result.Trials[0].Error)
	}

	if fakePreparer.calls != 1 {
		t.Errorf("preparer called %d times, want 1", fakePreparer.calls)
	}

	if fakeChecker.calls != 1 {
		t.Errorf("checker called %d times, want 1", fakeChecker.calls)
	}

	if !fakePreparer.workspace.closed {
		t.Error("workspace was not closed")
	}

	resultFile := filepath.Join(outputDir, "single-trial.json")
	if _, err := os.Stat(resultFile); err != nil {
		t.Errorf("result file not created: %v", err)
	}
}

func TestRun_MultipleTrials(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := createTestScenario(t, tmpDir, "multi-trial", 3)

	fakePreparer := &fakePreparer{
		workspace: &fakeWorkspace{dir: tmpDir},
	}
	fakeChecker := &fakeChecker{
		comparison: model.Comparison{
			SchemaVersion: model.CurrentSchemaVersion,
			Verdict:       model.VerdictFail,
		},
	}

	result, err := Run(context.Background(), RunOptions{
		ScenarioDir: scenarioDir,
		Preparer:    fakePreparer,
		Checker:     fakeChecker,
	})

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Trials) != 3 {
		t.Fatalf("len(Trials) = %d, want 3", len(result.Trials))
	}

	for i, tr := range result.Trials {
		if tr.Trial != i+1 {
			t.Errorf("Trial[%d].Trial = %d, want %d", i, tr.Trial, i+1)
		}
		if tr.Verdict != model.VerdictFail {
			t.Errorf("Trial[%d].Verdict = %q, want %q", i, tr.Verdict, model.VerdictFail)
		}
	}

	if fakePreparer.calls != 3 {
		t.Errorf("preparer called %d times, want 3", fakePreparer.calls)
	}

	if fakeChecker.calls != 3 {
		t.Errorf("checker called %d times, want 3", fakeChecker.calls)
	}
}

func TestRun_CheckerError(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := createTestScenario(t, tmpDir, "error-trial", 2)

	fakePreparer := &fakePreparer{
		workspace: &fakeWorkspace{dir: tmpDir},
	}

	callCount := 0
	fakeChecker := &fakeChecker{}
	originalRunCheck := func(_ context.Context, _ CheckRequest) (model.Comparison, error) {
		callCount++
		if callCount == 1 {
			return model.Comparison{}, errors.New("check failed")
		}
		return model.Comparison{
			SchemaVersion: model.CurrentSchemaVersion,
			Verdict:       model.VerdictPass,
		}, nil
	}
	fakeChecker.RunCheck = originalRunCheck

	result, err := Run(context.Background(), RunOptions{
		ScenarioDir: scenarioDir,
		Preparer:    fakePreparer,
		Checker:     fakeChecker,
	})

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Trials) != 2 {
		t.Fatalf("len(Trials) = %d, want 2", len(result.Trials))
	}

	if result.Trials[0].Error == "" {
		t.Errorf("Trial[0].Error is empty, want error message")
	}

	if result.Trials[1].Error != "" {
		t.Errorf("Trial[1].Error = %q, want empty", result.Trials[1].Error)
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := createTestScenario(t, tmpDir, "cancel-trial", 10)

	ctx, cancel := context.WithCancel(context.Background())

	fakePreparer := &fakePreparer{
		workspace: &fakeWorkspace{dir: tmpDir},
	}

	callCount := 0
	fakeChecker := &fakeChecker{}
	fakeChecker.RunCheck = func(_ context.Context, _ CheckRequest) (model.Comparison, error) {
		callCount++
		if callCount == 2 {
			cancel()
		}
		return model.Comparison{
			SchemaVersion: model.CurrentSchemaVersion,
			Verdict:       model.VerdictPass,
		}, nil
	}

	result, err := Run(ctx, RunOptions{
		ScenarioDir: scenarioDir,
		Preparer:    fakePreparer,
		Checker:     fakeChecker,
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run error = %v, want context.Canceled", err)
	}

	if len(result.Trials) != 2 {
		t.Errorf("len(Trials) = %d, want 2 (before cancellation)", len(result.Trials))
	}
}

func TestRun_MissingPreparer(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := createTestScenario(t, tmpDir, "no-preparer", 1)

	_, err := Run(context.Background(), RunOptions{
		ScenarioDir: scenarioDir,
		Checker:     &fakeChecker{},
	})

	if err == nil {
		t.Fatal("Run succeeded without preparer, want error")
	}
}

func TestRun_MissingChecker(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := createTestScenario(t, tmpDir, "no-checker", 1)

	_, err := Run(context.Background(), RunOptions{
		ScenarioDir: scenarioDir,
		Preparer:    &fakePreparer{workspace: &fakeWorkspace{dir: tmpDir}},
	})

	if err == nil {
		t.Fatal("Run succeeded without checker, want error")
	}
}

func TestRun_PreparerError(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := createTestScenario(t, tmpDir, "prep-error", 2)

	fakePreparer := &fakePreparer{
		err: errors.New("prepare failed"),
	}
	fakeChecker := &fakeChecker{
		comparison: model.Comparison{
			SchemaVersion: model.CurrentSchemaVersion,
			Verdict:       model.VerdictPass,
		},
	}

	result, err := Run(context.Background(), RunOptions{
		ScenarioDir: scenarioDir,
		Preparer:    fakePreparer,
		Checker:     fakeChecker,
	})

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Trials) != 2 {
		t.Fatalf("len(Trials) = %d, want 2", len(result.Trials))
	}

	for i, tr := range result.Trials {
		if tr.Error == "" {
			t.Errorf("Trial[%d].Error is empty, want error message", i)
		}
	}

	if fakeChecker.calls != 0 {
		t.Errorf("checker called %d times, want 0 (preparer failed)", fakeChecker.calls)
	}
}

func createTestScenario(t *testing.T, baseDir, id string, trials int) string {
	t.Helper()

	scenarioDir := filepath.Join(baseDir, id)
	baseCodeDir := filepath.Join(scenarioDir, "base")

	if err := os.MkdirAll(baseCodeDir, 0o755); err != nil {
		t.Fatalf("create base dir: %v", err)
	}

	goMod := "module example.com/testmod\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(baseCodeDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	mainGo := "package testmod\n\nfunc Add(a, b int) int { return a + b }\n"
	if err := os.WriteFile(filepath.Join(baseCodeDir, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	patchContent := `diff --git a/main.go b/main.go
index 1234567..abcdefg 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
 package testmod

-func Add(a, b int) int { return a + b }
+func Add(a, b int) int { return a + b + 0 }
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "change.patch"), []byte(patchContent), 0o644); err != nil {
		t.Fatalf("write patch: %v", err)
	}

	scenarioYAML := "id: " + id + "\ndescription: Test scenario\ntrials: "
	if trials < 10 {
		scenarioYAML += string(rune(trials + '0'))
	} else {
		scenarioYAML += "10"
	}
	scenarioYAML += "\n"

	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(scenarioYAML), 0o644); err != nil {
		t.Fatalf("write scenario.yaml: %v", err)
	}

	return scenarioDir
}

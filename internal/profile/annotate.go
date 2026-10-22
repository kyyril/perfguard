package profile

import (
	"path/filepath"
	"strings"

	pprof "github.com/google/pprof/profile"
	"github.com/kyyril/perfguard/internal/model"
)

// ExtractFunctionFiles builds a map of function name to source file path from a profile.
func ExtractFunctionFiles(prof *pprof.Profile) map[string]string {
	res := make(map[string]string)
	if prof == nil {
		return res
	}

	for _, fn := range prof.Function {
		if fn != nil && fn.Name != "" && fn.Filename != "" {
			res[fn.Name] = fn.Filename
		}
	}

	return res
}

// MatchesChangedFile checks whether sourceFile matches any path in changedFiles.
func MatchesChangedFile(sourceFile string, changedFiles []string) bool {
	if sourceFile == "" || len(changedFiles) == 0 {
		return false
	}

	normSource := filepath.ToSlash(sourceFile)

	for _, cf := range changedFiles {
		if cf == "" {
			continue
		}
		normChanged := filepath.ToSlash(cf)

		// Exact match
		if normSource == normChanged {
			return true
		}

		// Suffix match (e.g. /home/user/repo/pkg/work.go matches pkg/work.go)
		if strings.HasSuffix(normSource, "/"+normChanged) {
			return true
		}
	}

	return false
}

// AnnotateTouched marks candidate functions as Touched if their source file is in changedFiles.
func AnnotateTouched(candidates []model.Candidate, fnFiles map[string]string, changedFiles []string) []model.Candidate {
	if len(candidates) == 0 {
		return nil
	}

	annotated := make([]model.Candidate, len(candidates))
	copy(annotated, candidates)

	for i := range annotated {
		sourceFile := fnFiles[annotated[i].Function]
		annotated[i].Touched = MatchesChangedFile(sourceFile, changedFiles)
	}

	return annotated
}

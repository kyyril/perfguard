package runner

import (
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseCPUSet(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		want    []int
		wantErr bool
	}{
		{
			name: "single cpu",
			expr: "2",
			want: []int{2},
		},
		{
			name: "range of cpus",
			expr: "1-4",
			want: []int{1, 2, 3, 4},
		},
		{
			name: "comma separated",
			expr: "0, 2, 5",
			want: []int{0, 2, 5},
		},
		{
			name: "combined range and single",
			expr: "0-1, 4, 6-7",
			want: []int{0, 1, 4, 6, 7},
		},
		{
			name: "deduplicates overlap",
			expr: "0-2, 2-3",
			want: []int{0, 1, 2, 3},
		},
		{
			name:    "empty string",
			expr:    "",
			want:    nil,
			wantErr: false,
		},
		{
			name:    "invalid non-number",
			expr:    "abc",
			wantErr: true,
		},
		{
			name:    "inverted range",
			expr:    "4-2",
			wantErr: true,
		},
		{
			name:    "negative number",
			expr:    "-1",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCPUSet(tc.expr)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseCPUSet(%q) error = %v, wantErr %v", tc.expr, err, tc.wantErr)
			}
			if !tc.wantErr {
				if diff := cmp.Diff(tc.want, got); diff != "" {
					t.Errorf("parseCPUSet mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestCheckPreflightPinningOnNonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("skipping non-linux test on linux")
	}

	res := CheckPreflight(PreflightOptions{
		NumCPU:  4,
		PinCPUs: "0-1",
	})

	if res.PinWarning == "" {
		t.Error("expected non-empty PinWarning on non-Linux")
	}
	if !strings.Contains(res.PinWarning, "only supported on Linux") {
		t.Errorf("expected warning to mention only supported on Linux, got: %s", res.PinWarning)
	}

	foundInWarnings := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "only supported on Linux") {
			foundInWarnings = true
			break
		}
	}
	if !foundInWarnings {
		t.Error("expected PinWarning to be included in Warnings slice")
	}
}

func TestCheckPreflightHostExecution(t *testing.T) {
	res := CheckPreflight(PreflightOptions{})
	if res.Diagnostics.Load1 < 0 {
		t.Errorf("unexpected negative load1: %v", res.Diagnostics.Load1)
	}
}

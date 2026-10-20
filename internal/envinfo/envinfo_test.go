package envinfo_test

import (
	"math"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kyyril/perfguard/internal/envinfo"
)

func TestCollectorWithFakeLinuxFS(t *testing.T) {
	fakeFS := fstest.MapFS{
		"proc/cpuinfo": &fstest.MapFile{
			Data: []byte("processor\t: 0\nmodel name\t: AMD EPYC 7763 64-Core Processor\nflags\t: fpu vme\n"),
		},
		"proc/loadavg": &fstest.MapFile{
			Data: []byte("0.50 0.35 0.20 1/120 4567\n"),
		},
		"proc/version": &fstest.MapFile{
			Data: []byte("Linux version 6.1.0-21-amd64 (debian-kernel@lists.debian.org)\n"),
		},
		"sys/devices/system/cpu/cpu0/cpufreq/scaling_governor": &fstest.MapFile{
			Data: []byte("performance\n"),
		},
		"sys/fs/cgroup/cpu.max": &fstest.MapFile{
			Data: []byte("200000 100000\n"),
		},
	}

	c := envinfo.New(fakeFS)
	env := c.Fingerprint()

	if env.CPUModel != "AMD EPYC 7763 64-Core Processor" {
		t.Errorf("expected CPU model AMD EPYC, got %q", env.CPUModel)
	}
	if !strings.Contains(env.KernelVersion, "6.1.0-21-amd64") {
		t.Errorf("expected kernel version to contain 6.1.0-21-amd64, got %q", env.KernelVersion)
	}
	if env.Extra["governor"] != "performance" {
		t.Errorf("expected governor performance, got %q", env.Extra["governor"])
	}
	if env.Extra["load_avg"] != "0.50 0.35 0.20" {
		t.Errorf("expected load_avg '0.50 0.35 0.20', got %q", env.Extra["load_avg"])
	}
	if env.Extra["cgroup_cpu_limit"] != "2.00" {
		t.Errorf("expected cgroup_cpu_limit '2.00', got %q", env.Extra["cgroup_cpu_limit"])
	}

	preflight := c.CheckPreflight(4)
	if preflight.Governor != "performance" {
		t.Errorf("expected governor performance, got %q", preflight.Governor)
	}
	if math.Abs(preflight.Load1-0.50) > 1e-6 {
		t.Errorf("expected load1 0.50, got %v", preflight.Load1)
	}
	if math.Abs(preflight.CgroupCPULimit-2.00) > 1e-6 {
		t.Errorf("expected cgroup cpu limit 2.00, got %v", preflight.CgroupCPULimit)
	}
}

func TestPreflightWarnings(t *testing.T) {
	fakeFS := fstest.MapFS{
		"proc/loadavg": &fstest.MapFile{
			Data: []byte("9.50 8.00 6.00 2/250 8901\n"),
		},
		"sys/devices/system/cpu/cpu0/cpufreq/scaling_governor": &fstest.MapFile{
			Data: []byte("powersave\n"),
		},
		"sys/fs/cgroup/cpu.max": &fstest.MapFile{
			Data: []byte("200000 100000\n"),
		},
	}

	c := envinfo.New(fakeFS)
	preflight := c.CheckPreflight(4)

	if len(preflight.Warnings) != 3 {
		t.Fatalf("expected 3 warnings, got %d: %v", len(preflight.Warnings), preflight.Warnings)
	}

	joined := strings.Join(preflight.Warnings, "; ")
	if !strings.Contains(joined, "powersave") {
		t.Errorf("expected governor warning, got: %s", joined)
	}
	if !strings.Contains(joined, "high system load") {
		t.Errorf("expected load warning, got: %s", joined)
	}
	if !strings.Contains(joined, "cgroup limits CPU quota") {
		t.Errorf("expected cgroup warning, got: %s", joined)
	}
}

func TestCgroupV1Parsing(t *testing.T) {
	fakeFS := fstest.MapFS{
		"sys/fs/cgroup/cpu/cpu.cfs_quota_us": &fstest.MapFile{
			Data: []byte("400000\n"),
		},
		"sys/fs/cgroup/cpu/cpu.cfs_period_us": &fstest.MapFile{
			Data: []byte("100000\n"),
		},
	}

	c := envinfo.New(fakeFS)
	preflight := c.CheckPreflight(8)
	if math.Abs(preflight.CgroupCPULimit-4.0) > 1e-6 {
		t.Errorf("expected cgroup v1 limit 4.0, got %v", preflight.CgroupCPULimit)
	}
}

func TestCgroupV2MaxUnlimited(t *testing.T) {
	fakeFS := fstest.MapFS{
		"sys/fs/cgroup/cpu.max": &fstest.MapFile{
			Data: []byte("max 100000\n"),
		},
	}

	c := envinfo.New(fakeFS)
	preflight := c.CheckPreflight(8)
	if preflight.CgroupCPULimit != 0 {
		t.Errorf("expected unlimited cgroup (0), got %v", preflight.CgroupCPULimit)
	}
}

func TestEmptyFSProducesEmptyFieldsWithoutErrors(t *testing.T) {
	fakeFS := fstest.MapFS{}

	c := envinfo.New(fakeFS)
	env := c.Fingerprint()

	if env.CPUModel != "" {
		t.Errorf("expected empty CPU model, got %q", env.CPUModel)
	}
	if env.KernelVersion != "" {
		t.Errorf("expected empty kernel version, got %q", env.KernelVersion)
	}

	preflight := c.CheckPreflight(4)
	if len(preflight.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %v", preflight.Warnings)
	}
	if preflight.Load1 != 0 || preflight.Governor != "" || preflight.CgroupCPULimit != 0 {
		t.Errorf("expected empty preflight, got %+v", preflight)
	}
}

func TestCPUInfoAlternativeFormats(t *testing.T) {
	tests := []struct {
		name      string
		cpuinfo   string
		wantModel string
	}{
		{
			name:      "ARM Model field",
			cpuinfo:   "processor\t: 0\nModel\t: Raspberry Pi 4 Model B Rev 1.4\n",
			wantModel: "Raspberry Pi 4 Model B Rev 1.4",
		},
		{
			name:      "Hardware field",
			cpuinfo:   "processor\t: 0\nHardware\t: BCM2835\n",
			wantModel: "BCM2835",
		},
		{
			name:      "Processor name field",
			cpuinfo:   "processor\t: 0\nProcessor\t: ARMv7 Processor rev 3\n",
			wantModel: "ARMv7 Processor rev 3",
		},
		{
			name:      "Digit only processor number skipped",
			cpuinfo:   "processor\t: 0\nprocessor\t: 1\nmodel name\t: Intel Core i5\n",
			wantModel: "Intel Core i5",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"proc/cpuinfo": &fstest.MapFile{Data: []byte(tc.cpuinfo)},
			}
			c := envinfo.New(fsys)
			env := c.Fingerprint()
			if env.CPUModel != tc.wantModel {
				t.Errorf("expected %q, got %q", tc.wantModel, env.CPUModel)
			}
		})
	}
}

func TestHostCollector(t *testing.T) {
	env := envinfo.Fingerprint()
	if env.OS == "" {
		t.Error("expected non-empty OS")
	}
	if env.Arch == "" {
		t.Error("expected non-empty Arch")
	}
	if env.GoVersion == "" {
		t.Error("expected non-empty GoVersion")
	}
	if env.NumCPU <= 0 {
		t.Errorf("expected NumCPU > 0, got %d", env.NumCPU)
	}
	if env.GOMAXPROCS <= 0 {
		t.Errorf("expected GOMAXPROCS > 0, got %d", env.GOMAXPROCS)
	}

	_ = envinfo.CheckPreflight(env.NumCPU)
}

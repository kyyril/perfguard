// Package envinfo collects hardware, operating system, and runtime metadata
// to fingerprint benchmark environments and detect noise factors.
package envinfo

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/kyyril/perfguard/internal/model"
)

// Preflight contains environment checks that might affect benchmark accuracy.
type Preflight struct {
	Load1          float64  `json:"load1"`
	Load5          float64  `json:"load5"`
	Load15         float64  `json:"load15"`
	Governor       string   `json:"governor,omitempty"`
	CgroupCPULimit float64  `json:"cgroup_cpu_limit,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

// Collector inspects system state and builds environment fingerprints.
type Collector struct {
	fsys fs.FS
}

// New creates a Collector. If fsys is nil, host files are accessed directly.
func New(fsys fs.FS) *Collector {
	return &Collector{fsys: fsys}
}

// DefaultCollector queries the host operating system.
var DefaultCollector = New(nil)

// Fingerprint collects the host environment fingerprint.
func Fingerprint() model.Environment {
	return DefaultCollector.Fingerprint()
}

// CheckPreflight runs preflight checks on the host environment.
func CheckPreflight(numCPU int) Preflight {
	return DefaultCollector.CheckPreflight(numCPU)
}

// Fingerprint inspects the configured filesystem and runtime to produce an Environment.
func (c *Collector) Fingerprint() model.Environment {
	numCPU := runtime.NumCPU()
	gomaxprocs := runtime.GOMAXPROCS(0)
	hostname, _ := os.Hostname()

	env := model.Environment{
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		GoVersion:     runtime.Version(),
		NumCPU:        numCPU,
		CPUModel:      c.readCPUModel(),
		GOMAXPROCS:    gomaxprocs,
		Hostname:      hostname,
		KernelVersion: c.readKernelVersion(),
		Extra:         make(map[string]string),
	}

	preflight := c.CheckPreflight(numCPU)
	if preflight.Governor != "" {
		env.Extra["governor"] = preflight.Governor
	}
	if preflight.Load1 > 0 || preflight.Load5 > 0 || preflight.Load15 > 0 {
		env.Extra["load_avg"] = fmt.Sprintf("%.2f %.2f %.2f", preflight.Load1, preflight.Load5, preflight.Load15)
	}
	if preflight.CgroupCPULimit > 0 {
		env.Extra["cgroup_cpu_limit"] = fmt.Sprintf("%.2f", preflight.CgroupCPULimit)
	}

	return env
}

// CheckPreflight evaluates system load, CPU governor, and container quotas.
func (c *Collector) CheckPreflight(numCPU int) Preflight {
	p := Preflight{}
	p.Load1, p.Load5, p.Load15 = c.readLoadAvg()
	p.Governor = c.readGovernor()
	p.CgroupCPULimit = c.readCgroupCPULimit()

	if p.Governor != "" && p.Governor != "performance" {
		p.Warnings = append(p.Warnings, fmt.Sprintf("cpu governor is %q (performance recommended)", p.Governor))
	}
	if numCPU > 0 && p.Load1 > float64(numCPU) {
		p.Warnings = append(p.Warnings, fmt.Sprintf("high system load: 1m load average is %.2f with %d CPUs", p.Load1, numCPU))
	}
	if p.CgroupCPULimit > 0 && numCPU > 0 && p.CgroupCPULimit < float64(numCPU) {
		p.Warnings = append(p.Warnings, fmt.Sprintf("cgroup limits CPU quota to %.2f (host has %d CPUs)", p.CgroupCPULimit, numCPU))
	}

	return p
}

func (c *Collector) readFile(path string) ([]byte, error) {
	if c.fsys != nil {
		clean := strings.TrimPrefix(path, "/")
		return fs.ReadFile(c.fsys, clean)
	}
	return os.ReadFile(path)
}

func (c *Collector) readCPUModel() string {
	data, err := c.readFile("/proc/cpuinfo")
	if err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.ToLower(strings.TrimSpace(parts[0]))
				if key == "model name" || key == "model" || key == "cpu model" || key == "hardware" || key == "processor" {
					val := strings.TrimSpace(parts[1])
					if val != "" && !isDigitOnly(val) {
						return val
					}
				}
			}
		}
	}

	// Fallback on Windows if /proc is not available.
	if c.fsys == nil && runtime.GOOS == "windows" {
		if id := os.Getenv("PROCESSOR_IDENTIFIER"); id != "" {
			return id
		}
	}

	return ""
}

func (c *Collector) readKernelVersion() string {
	data, err := c.readFile("/proc/version")
	if err == nil {
		line := strings.TrimSpace(string(data))
		if line != "" {
			return line
		}
	}
	return ""
}

func (c *Collector) readLoadAvg() (l1, l5, l15 float64) {
	data, err := c.readFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		l1, _ = strconv.ParseFloat(fields[0], 64)
		l5, _ = strconv.ParseFloat(fields[1], 64)
		l15, _ = strconv.ParseFloat(fields[2], 64)
	}
	return l1, l5, l15
}

func (c *Collector) readGovernor() string {
	paths := []string{
		"/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor",
		"/sys/devices/system/cpu/cpufreq/policy0/scaling_governor",
	}
	for _, p := range paths {
		data, err := c.readFile(p)
		if err == nil {
			gov := strings.TrimSpace(string(data))
			if gov != "" {
				return gov
			}
		}
	}
	return ""
}

func (c *Collector) readCgroupCPULimit() float64 {
	// Try cgroup v2: cpu.max contains "quota period" or "max period".
	if data, err := c.readFile("/sys/fs/cgroup/cpu.max"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 2 && fields[0] != "max" {
			quota, qErr := strconv.ParseFloat(fields[0], 64)
			period, pErr := strconv.ParseFloat(fields[1], 64)
			if qErr == nil && pErr == nil && period > 0 && quota > 0 {
				return quota / period
			}
		}
	}

	// Try cgroup v1: cpu.cfs_quota_us and cpu.cfs_period_us.
	quotaPaths := []string{
		"/sys/fs/cgroup/cpu/cpu.cfs_quota_us",
		"/sys/fs/cgroup/cpu,cpuacct/cpu.cfs_quota_us",
	}
	periodPaths := []string{
		"/sys/fs/cgroup/cpu/cpu.cfs_period_us",
		"/sys/fs/cgroup/cpu,cpuacct/cpu.cfs_period_us",
	}

	for i := range quotaPaths {
		qData, qErr := c.readFile(quotaPaths[i])
		pData, pErr := c.readFile(periodPaths[i])
		if qErr == nil && pErr == nil {
			quota, err1 := strconv.ParseFloat(strings.TrimSpace(string(qData)), 64)
			period, err2 := strconv.ParseFloat(strings.TrimSpace(string(pData)), 64)
			if err1 == nil && err2 == nil && period > 0 && quota > 0 {
				return quota / period
			}
		}
	}

	return 0
}

func isDigitOnly(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

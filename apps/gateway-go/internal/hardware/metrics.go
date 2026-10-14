package hardware

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type SystemMetrics struct {
	CPU     MetricValue `json:"cpu"`
	Memory  MetricValue `json:"memory"`
	Storage MetricValue `json:"storage"`
}

type ProcessMetrics struct {
	MemoryBytes uint64 `json:"memoryBytes"`
	DiskBytes   uint64 `json:"diskBytes"`
	DiskPath    string `json:"diskPath,omitempty"`
}

type MetricValue struct {
	UsedPercent float64 `json:"usedPercent"`
	UsedBytes   uint64  `json:"usedBytes,omitempty"`
	TotalBytes  uint64  `json:"totalBytes,omitempty"`
}

type cpuSample struct {
	idle  uint64
	total uint64
}

func ReadSystemMetrics() SystemMetrics {
	first, ok := readCPUSample()
	if ok {
		time.Sleep(120 * time.Millisecond)
	}
	second, secondOK := readCPUSample()
	memory := readMemoryMetric()
	storage := readStorageMetric("/")
	metrics := SystemMetrics{Memory: memory, Storage: storage}
	if ok && secondOK && second.total > first.total {
		total := second.total - first.total
		idle := second.idle - first.idle
		if total > 0 && total >= idle {
			metrics.CPU.UsedPercent = float64(total-idle) * 100 / float64(total)
		}
	}
	return metrics
}

func ReadProcessMetrics() ProcessMetrics {
	result := ProcessMetrics{}
	if executable, err := os.Executable(); err == nil {
		result.DiskPath = executable
		if info, statErr := os.Stat(executable); statErr == nil {
			result.DiskBytes = uint64(info.Size())
		}
	}
	if raw, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "VmRSS:" {
				if value, parseErr := strconv.ParseUint(fields[1], 10, 64); parseErr == nil {
					result.MemoryBytes = value * 1024
				}
				break
			}
		}
	}
	return result
}

func readCPUSample() (cpuSample, bool) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return cpuSample{}, false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return cpuSample{}, false
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuSample{}, false
	}
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuSample{}, false
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return cpuSample{idle: idle, total: total}, true
}

func readMemoryMetric() MetricValue {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return MetricValue{}
	}
	values := map[string]uint64{}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[key] = value * 1024
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	if total == 0 || available > total {
		return MetricValue{}
	}
	used := total - available
	return MetricValue{UsedPercent: float64(used) * 100 / float64(total), UsedBytes: used, TotalBytes: total}
}

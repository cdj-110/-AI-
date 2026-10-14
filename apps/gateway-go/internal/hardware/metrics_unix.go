//go:build !windows

package hardware

import "syscall"

func readStorageMetric(path string) MetricValue {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return MetricValue{}
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if total == 0 || free > total {
		return MetricValue{}
	}
	used := total - free
	return MetricValue{UsedPercent: float64(used) * 100 / float64(total), UsedBytes: used, TotalBytes: total}
}

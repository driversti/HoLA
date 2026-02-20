package metrics

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

type SystemMetrics struct {
	Hostname      string       `json:"hostname"`
	UptimeSeconds uint64       `json:"uptime_seconds"`
	CPU           CPUMetrics   `json:"cpu"`
	Memory        MemMetrics   `json:"memory"`
	Disk          []DiskMetric `json:"disk"`
}

type CPUMetrics struct {
	UsagePercent float64 `json:"usage_percent"`
	Cores        int     `json:"cores"`
}

type MemMetrics struct {
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

type DiskMetric struct {
	MountPoint   string  `json:"mount_point"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// Collect gathers current system metrics.
func Collect(ctx context.Context) (*SystemMetrics, error) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}

	cpuPercent, err := cpu.PercentWithContext(ctx, 500*time.Millisecond, false)
	if err != nil {
		return nil, err
	}
	cores, err := cpu.CountsWithContext(ctx, true)
	if err != nil {
		return nil, err
	}

	vmem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}

	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, err
	}

	var disks []DiskMetric
	for _, p := range partitions {
		usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil || usage.Total == 0 {
			continue
		}
		disks = append(disks, DiskMetric{
			MountPoint:   p.Mountpoint,
			TotalBytes:   usage.Total,
			UsedBytes:    usage.Used,
			UsagePercent: usage.UsedPercent,
		})
	}

	var cpuUsage float64
	if len(cpuPercent) > 0 {
		cpuUsage = cpuPercent[0]
	}

	return &SystemMetrics{
		Hostname:      info.Hostname,
		UptimeSeconds: info.Uptime,
		CPU: CPUMetrics{
			UsagePercent: cpuUsage,
			Cores:        cores,
		},
		Memory: MemMetrics{
			TotalBytes:   vmem.Total,
			UsedBytes:    vmem.Used,
			UsagePercent: vmem.UsedPercent,
		},
		Disk: disks,
	}, nil
}

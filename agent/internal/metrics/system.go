package metrics

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/sensors"
)

type SystemMetrics struct {
	Hostname      string       `json:"hostname"`
	UptimeSeconds uint64       `json:"uptime_seconds"`
	CPU           CPUMetrics   `json:"cpu"`
	Memory        MemMetrics   `json:"memory"`
	Disk          []DiskMetric `json:"disk"`
}

type CPUMetrics struct {
	UsagePercent       float64  `json:"usage_percent"`
	Cores              int      `json:"cores"`
	TemperatureCelsius *float64 `json:"temperature_celsius,omitempty"`
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

// selectCPUTemperature picks the best CPU temperature from available sensors.
// Priority: package(5) > tdie(4) > tctl/cpu_thermal/cpu-thermal(3) > *cpu*/core*(2) > first valid(1).
// Readings ≤0 or >150°C are skipped as invalid.
func selectCPUTemperature(temps []sensors.TemperatureStat) *float64 {
	var bestTemp float64
	var bestPriority int

	for _, t := range temps {
		if t.Temperature <= 0 || t.Temperature > 150 {
			continue
		}

		key := strings.ToLower(t.SensorKey)
		var priority int

		switch {
		case strings.Contains(key, "package"):
			priority = 5
		case strings.Contains(key, "tdie"):
			priority = 4
		case strings.Contains(key, "tctl"),
			strings.Contains(key, "cpu_thermal"),
			strings.Contains(key, "cpu-thermal"):
			priority = 3
		case strings.Contains(key, "cpu"),
			strings.Contains(key, "core"):
			priority = 2
		default:
			priority = 1
		}

		if priority > bestPriority {
			bestPriority = priority
			bestTemp = t.Temperature
		}
	}

	if bestPriority == 0 {
		return nil
	}
	return &bestTemp
}

func cpuTemperature(ctx context.Context) *float64 {
	temps, err := sensors.TemperaturesWithContext(ctx)
	if err != nil {
		slog.Debug("failed to read CPU temperature", "error", err)
		return nil
	}
	return selectCPUTemperature(temps)
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
			UsagePercent:       cpuUsage,
			Cores:              cores,
			TemperatureCelsius: cpuTemperature(ctx),
		},
		Memory: MemMetrics{
			TotalBytes:   vmem.Total,
			UsedBytes:    vmem.Used,
			UsagePercent: vmem.UsedPercent,
		},
		Disk: disks,
	}, nil
}

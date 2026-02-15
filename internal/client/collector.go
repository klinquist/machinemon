package client

import (
	"fmt"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

type SystemMetrics struct {
	CPUPercent     float64
	MemPercent     float64
	MemTotal       uint64
	MemUsed        uint64
	DiskPercent    float64
	DiskTotal      uint64
	DiskUsed       uint64
}

// CollectSystemMetrics gathers CPU (1-second sample), memory, and root disk usage.
func CollectSystemMetrics() (*SystemMetrics, error) {
	cpuPcts, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("cpu: %w", err)
	}
	cpuPct := 0.0
	if len(cpuPcts) > 0 {
		cpuPct = cpuPcts[0]
	}

	vmem, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("memory: %w", err)
	}

	diskPath := "/"
	if runtime.GOOS == "windows" {
		diskPath = "C:\\"
	}
	diskStat, err := disk.Usage(diskPath)
	if err != nil {
		return nil, fmt.Errorf("disk: %w", err)
	}

	return &SystemMetrics{
		CPUPercent:  cpuPct,
		MemPercent:  vmem.UsedPercent,
		MemTotal:    vmem.Total,
		MemUsed:     vmem.Used,
		DiskPercent: diskStat.UsedPercent,
		DiskTotal:   diskStat.Total,
		DiskUsed:    diskStat.Used,
	}, nil
}

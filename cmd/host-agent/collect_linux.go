//go:build linux

package main

import (
	"context"
	"errors"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/inventory"
)

type cpuTimes struct{ total, idle uint64 }

func parseCPU(body []byte) (cpuTimes, error) {
	line := strings.SplitN(string(body), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTimes{}, errors.New("invalid /proc/stat CPU line")
	}
	var sample cpuTimes
	for i, field := range fields[1:] {
		if i >= 8 {
			continue
		} // guest time is already included in user/nice
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuTimes{}, err
		}
		sample.total += value
		if i == 3 || i == 4 {
			sample.idle += value
		}
	}
	return sample, nil
}

func cpuPercent(before, after cpuTimes) (float64, error) {
	if after.total <= before.total || after.idle < before.idle {
		return 0, errors.New("CPU counters did not advance")
	}
	busy := (after.total - before.total) - (after.idle - before.idle)
	return 100 * float64(busy) / float64(after.total-before.total), nil
}

func parseMemory(body []byte) (percent float64, capacityGB int, err error) {
	values := map[string]uint64{}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		if key != "MemTotal" && key != "MemAvailable" {
			continue
		}
		value, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			return 0, 0, parseErr
		}
		values[key] = value
	}
	total, available := values["MemTotal"], values["MemAvailable"]
	if total == 0 || available > total {
		return 0, 0, errors.New("invalid /proc/meminfo")
	}
	return 100 * float64(total-available) / float64(total), int(math.Round(float64(total*1024) / 1e9)), nil
}

func collect(ctx context.Context) (inventory.HostReport, error) {
	first, err := os.ReadFile("/proc/stat")
	if err != nil {
		return inventory.HostReport{}, err
	}
	before, err := parseCPU(first)
	if err != nil {
		return inventory.HostReport{}, err
	}
	select {
	case <-ctx.Done():
		return inventory.HostReport{}, ctx.Err()
	case <-time.After(time.Second):
	}
	second, err := os.ReadFile("/proc/stat")
	if err != nil {
		return inventory.HostReport{}, err
	}
	after, err := parseCPU(second)
	if err != nil {
		return inventory.HostReport{}, err
	}
	cpu, err := cpuPercent(before, after)
	if err != nil {
		return inventory.HostReport{}, err
	}
	memory, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return inventory.HostReport{}, err
	}
	ram, ramGB, err := parseMemory(memory)
	if err != nil {
		return inventory.HostReport{}, err
	}
	var disk syscall.Statfs_t
	if err := syscall.Statfs("/", &disk); err != nil {
		return inventory.HostReport{}, err
	}
	if disk.Blocks == 0 || disk.Bavail > disk.Blocks {
		return inventory.HostReport{}, errors.New("invalid root filesystem size")
	}
	diskPercent := 100 * float64(disk.Blocks-disk.Bavail) / float64(disk.Blocks)
	sample := inventory.HostReport{CPUPercent: cpu, RAMPercent: ram, DiskPercent: diskPercent, VCPU: runtime.NumCPU(), RAMGB: ramGB}
	return sample, sample.Validate()
}

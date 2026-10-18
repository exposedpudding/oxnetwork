//go:build windows

package main

import (
	"context"
	"errors"
	"math"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/exposedpudding/oxnetwork/internal/inventory"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var getSystemTimes = kernel32.NewProc("GetSystemTimes")
var globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
var getDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")

type fileTime struct{ Low, High uint32 }

func (v fileTime) uint64() uint64 { return uint64(v.High)<<32 | uint64(v.Low) }

type systemTimes struct{ idle, kernel, user uint64 }

func readSystemTimes() (systemTimes, error) {
	var idle, kernel, user fileTime
	ok, _, err := getSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if ok == 0 {
		return systemTimes{}, err
	}
	return systemTimes{idle.uint64(), kernel.uint64(), user.uint64()}, nil
}

type memoryStatusEx struct {
	Length, MemoryLoad           uint32
	TotalPhys, AvailPhys         uint64
	TotalPageFile, AvailPageFile uint64
	TotalVirtual, AvailVirtual   uint64
	AvailExtendedVirtual         uint64
}

func collect(ctx context.Context) (inventory.HostReport, error) {
	before, err := readSystemTimes()
	if err != nil {
		return inventory.HostReport{}, err
	}
	select {
	case <-ctx.Done():
		return inventory.HostReport{}, ctx.Err()
	case <-time.After(time.Second):
	}
	after, err := readSystemTimes()
	if err != nil {
		return inventory.HostReport{}, err
	}
	if after.kernel < before.kernel || after.user < before.user || after.idle < before.idle {
		return inventory.HostReport{}, errors.New("Windows CPU counters went backwards")
	}
	total := (after.kernel - before.kernel) + (after.user - before.user)
	idle := after.idle - before.idle
	if total == 0 || idle > total {
		return inventory.HostReport{}, errors.New("invalid Windows CPU counters")
	}
	cpu := 100 * float64(total-idle) / float64(total)
	memory := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	ok, _, err := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memory)))
	if ok == 0 {
		return inventory.HostReport{}, err
	}
	if memory.TotalPhys == 0 || memory.AvailPhys > memory.TotalPhys {
		return inventory.HostReport{}, errors.New("invalid Windows memory counters")
	}
	ram := 100 * float64(memory.TotalPhys-memory.AvailPhys) / float64(memory.TotalPhys)
	ramGB := int(math.Round(float64(memory.TotalPhys) / 1e9))
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	root, err := syscall.UTF16PtrFromString(drive + `\`)
	if err != nil {
		return inventory.HostReport{}, err
	}
	var available, diskTotal uint64
	ok, _, err = getDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(root)), uintptr(unsafe.Pointer(&available)), uintptr(unsafe.Pointer(&diskTotal)), 0)
	if ok == 0 {
		return inventory.HostReport{}, err
	}
	if diskTotal == 0 || available > diskTotal {
		return inventory.HostReport{}, errors.New("invalid Windows system drive counters")
	}
	disk := 100 * float64(diskTotal-available) / float64(diskTotal)
	sample := inventory.HostReport{CPUPercent: cpu, RAMPercent: ram, DiskPercent: disk, VCPU: runtime.NumCPU(), RAMGB: ramGB}
	return sample, sample.Validate()
}

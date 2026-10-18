//go:build linux

package main

import (
	"math"
	"testing"
)

func TestProcSamples(t *testing.T) {
	before, err := parseCPU([]byte("cpu  100 0 50 850 0 0 0 0\ncpu0 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	after, err := parseCPU([]byte("cpu  130 0 60 910 0 0 0 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	usage, err := cpuPercent(before, after)
	if err != nil || math.Abs(usage-40) > 0.01 {
		t.Fatalf("CPU = %v, %v", usage, err)
	}
	ram, capacity, err := parseMemory([]byte("MemTotal:  16000000 kB\nMemFree: 100 kB\nMemAvailable: 4000000 kB\n"))
	if err != nil || ram != 75 || capacity != 16 {
		t.Fatalf("RAM = %v, capacity = %d, %v", ram, capacity, err)
	}
}

func TestReportURLRequiresPrivateTransportAndStableNodeID(t *testing.T) {
	if _, err := reportURL("https://oxnet.work/internal/v1/host-report", "tailscale-123"); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"http://oxnet.work/internal/v1/host-report", "https://oxnet.work/other"} {
		if _, err := reportURL(endpoint, "tailscale-123"); err == nil {
			t.Fatalf("accepted %s", endpoint)
		}
	}
	if _, err := reportURL("https://oxnet.work/internal/v1/host-report", "../other"); err == nil {
		t.Fatal("accepted traversal")
	}
}

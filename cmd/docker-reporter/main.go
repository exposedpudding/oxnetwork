// docker-reporter sends a node's container list to oxnet.work (ADR-019). It is the only
// oxnet.work program that touches the Docker socket, and it makes exactly one kind of
// request on it: GET /containers/json. It opens no listener, runs no container action
// and forwards only an allow-listed subset of fields (no environment, mounts, command
// lines or labels other than two Compose names).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/inventory"
)

var nodeIDPattern = regexp.MustCompile(`^tailscale-[0-9]+$`)

func reportURL(base, nodeID string) (string, error) {
	if !nodeIDPattern.MatchString(nodeID) {
		return "", fmt.Errorf("invalid Tailscale node ID")
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/internal/v1/docker-report" {
		return "", fmt.Errorf("invalid container report endpoint")
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "http" || ip == nil || !ip.IsLoopback() {
			return "", fmt.Errorf("container report endpoint requires HTTPS or loopback HTTP")
		}
	}
	return base + "/" + nodeID, nil
}

func send(ctx context.Context, client *http.Client, endpoint, token string, report inventory.ContainerReport) error {
	body, err := json.Marshal(report)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("container report returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func main() {
	base := flag.String("endpoint", "https://oxnet.work/internal/v1/docker-report", "private control-plane report endpoint")
	nodeID := flag.String("node-id", "", "Tailscale inventory node ID, for example tailscale-123")
	tokenFile := flag.String("token-file", "", "read-only container report token file (not the host report token)")
	socket := flag.String("docker-socket", "/var/run/docker.sock", "Docker Engine Unix socket")
	interval := flag.Duration("interval", 30*time.Second, "time between reports (15s to 5m)")
	once := flag.Bool("once", false, "collect and send one report, then exit")
	flag.Parse()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	endpoint, err := reportURL(*base, *nodeID)
	if *tokenFile == "" || err != nil || *interval < 15*time.Second || *interval > 5*time.Minute {
		log.Error("node ID, token file, private HTTPS or loopback endpoint and a 15s-5m interval are required", "error", err)
		os.Exit(1)
	}
	tokenBody, err := os.ReadFile(*tokenFile)
	if err != nil {
		log.Error("token file unavailable", "error", err)
		os.Exit(1)
	}
	token := strings.TrimSpace(string(tokenBody))
	if len(token) < 32 {
		log.Error("token is too short")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	client := &http.Client{Timeout: 10 * time.Second}
	for {
		list, err := listContainers(ctx, *socket)
		if err == nil {
			report, skipped := toReport(list)
			if skipped > 0 {
				log.Warn("some containers were not reported", "skipped", skipped)
			}
			err = send(ctx, client, endpoint, token, report)
		}
		if err != nil && ctx.Err() == nil {
			log.Warn("container report failed", "error", err)
		}
		if *once {
			if err != nil {
				os.Exit(1)
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(*interval):
		}
	}
}

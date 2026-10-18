// host-agent sends read-only host samples to oxnet.work over private HTTPS or
// a local loopback port. It never opens a listener or accesses Docker.
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
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/internal/v1/host-report" {
		return "", fmt.Errorf("invalid host report endpoint")
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "http" || ip == nil || !ip.IsLoopback() {
			return "", fmt.Errorf("host report endpoint requires HTTPS or loopback HTTP")
		}
	}
	return base + "/" + nodeID, nil
}

func send(ctx context.Context, client *http.Client, endpoint, token string, sample inventory.HostReport) error {
	body, err := json.Marshal(sample)
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
		return fmt.Errorf("host report returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func main() {
	base := flag.String("endpoint", "https://oxnet.work/internal/v1/host-report", "private control-plane report endpoint")
	nodeID := flag.String("node-id", "", "Tailscale inventory node ID, for example tailscale-123")
	tokenFile := flag.String("token-file", "", "read-only host report token file")
	logFile := flag.String("log-file", "", "optional append-only diagnostic log")
	once := flag.Bool("once", false, "collect and send one report, then exit")
	flag.Parse()
	var logOutput io.Writer = os.Stdout
	if *logFile != "" {
		file, err := os.OpenFile(*logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(1)
		}
		defer file.Close()
		logOutput = file
	}
	log := slog.New(slog.NewJSONHandler(logOutput, nil))
	endpoint, err := reportURL(*base, *nodeID)
	if *tokenFile == "" || err != nil {
		log.Error("node ID, token file, and private HTTPS or loopback endpoint are required", "error", err)
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
		sample, err := collect(ctx)
		if err == nil {
			err = send(ctx, client, endpoint, token, sample)
		}
		if err != nil && ctx.Err() == nil {
			log.Warn("host sample failed", "error", err)
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
		case <-time.After(30 * time.Second):
		}
	}
}

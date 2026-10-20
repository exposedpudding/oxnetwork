package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"encoding/json"

	"github.com/exposedpudding/oxnetwork/internal/inventory"
)

// maxDockerResponse bounds what is read from the Docker socket.
const maxDockerResponse = 8 << 20

// dockerContainer is the subset of the Docker Engine "list containers" response this
// program reads. Anything else in the response (labels, mounts, networks, command) is
// ignored by the decoder and never copied out. The listing does not include environment
// variables, but the allow-list is what keeps it that way if Docker ever adds them.
type dockerContainer struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	Created int64    `json:"Created"`
	State   string   `json:"State"`
	Status  string   `json:"Status"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
	Labels map[string]string `json:"Labels"`
}

// listContainers issues the single request this program is allowed to make:
// GET /containers/json?all=1 on the Docker socket. The path is a constant; nothing from
// configuration or a response is ever used to build another request.
func listContainers(ctx context.Context, socket string) ([]dockerContainer, error) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socket)
		},
		DisableKeepAlives: true,
	}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/json?all=1", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("docker returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDockerResponse+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxDockerResponse {
		return nil, errors.New("docker response is too large")
	}
	var list []dockerContainer
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("docker response is not a container list: %w", err)
	}
	return list, nil
}

var knownStates = map[string]bool{"created": true, "running": true, "paused": true, "restarting": true, "removing": true, "exited": true, "dead": true}

func clean(value string, max int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	if len(value) > max {
		value = value[:max]
	}
	return value
}

// healthFrom reads the health result from the status text ("Up 2 hours (healthy)"),
// the only place the list endpoint exposes it.
func healthFrom(status string) string {
	switch {
	case strings.Contains(status, "(unhealthy)"):
		return "unhealthy"
	case strings.Contains(status, "(healthy)"):
		return "healthy"
	case strings.Contains(status, "(health: starting)"):
		return "starting"
	}
	return ""
}

// toReport maps Docker's list to the report the server accepts. It returns the number
// of containers skipped because they could not be represented (unknown state or an
// unusable id), so the caller can log it instead of failing the whole report.
func toReport(list []dockerContainer) (inventory.ContainerReport, int) {
	report := inventory.ContainerReport{Containers: []inventory.Container{}}
	skipped := 0
	for index, c := range list {
		state := strings.ToLower(c.State)
		if len(c.ID) < 12 || !knownStates[state] {
			skipped++
			continue
		}
		id := c.ID[:12]
		name := id
		if len(c.Names) > 0 {
			if trimmed := clean(strings.TrimPrefix(c.Names[0], "/"), 128); trimmed != "" {
				name = trimmed
			}
		}
		image := clean(c.Image, 256)
		if strings.HasPrefix(image, "sha256:") && len(image) > 19 {
			image = image[:19]
		}
		if image == "" {
			image = "unknown"
		}
		var ports []string
		seen := map[string]bool{}
		for _, p := range c.Ports {
			if p.PublicPort <= 0 {
				continue
			}
			text := clean(fmt.Sprintf("%s:%d->%d/%s", p.IP, p.PublicPort, p.PrivatePort, p.Type), 64)
			if !seen[text] {
				seen[text] = true
				ports = append(ports, text)
			}
		}
		sort.Strings(ports)
		if len(ports) > 32 {
			ports = ports[:32]
		}
		item := inventory.Container{
			ID: id, Name: name, Image: image, State: state, Status: clean(c.Status, 128),
			Health: healthFrom(c.Status), CreatedAt: time.Unix(c.Created, 0).UTC(), Ports: ports,
		}
		// Labels are untrusted free text: keep a Compose name only if it is a plain name.
		if inventory.ValidComposeName(c.Labels["com.docker.compose.project"]) {
			item.ComposeProject = c.Labels["com.docker.compose.project"]
		}
		if inventory.ValidComposeName(c.Labels["com.docker.compose.service"]) {
			item.ComposeService = c.Labels["com.docker.compose.service"]
		}
		if ports == nil {
			item.Ports = []string{}
		}
		report.Containers = append(report.Containers, item)
		if len(report.Containers) == inventory.MaxContainersPerReport {
			skipped += len(list) - index - 1
			break
		}
	}
	return report, skipped
}

package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/domain"
)

const apiBase = "https://api.tailscale.com"

// Client reads the Tailscale control-plane device list. It does not manage
// devices or infer host metrics, links, or online state from inventory data.
type Client struct {
	Tailnet string
	APIKey  string
	HTTP    *http.Client
	BaseURL string // Tests only; production uses apiBase.
}

type device struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Hostname  string   `json:"hostname"`
	OS        string   `json:"os"`
	Addresses []string `json:"addresses"`
	LastSeen  string   `json:"lastSeen"`
}

func (c Client) Snapshot(ctx context.Context, now time.Time) (domain.Snapshot, error) {
	if c.Tailnet == "" || c.APIKey == "" {
		return domain.Snapshot{}, errors.New("Tailscale tailnet and API key are required")
	}
	base := c.BaseURL
	if base == "" {
		base = apiBase
	}
	endpoint := strings.TrimRight(base, "/") + "/api/v2/tailnet/" + url.PathEscape(c.Tailnet) + "/devices?fields=default"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.Snapshot{}, err
	}
	req.SetBasicAuth(c.APIKey, "")
	req.Header.Set("Accept", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("Tailscale device request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return domain.Snapshot{}, fmt.Errorf("Tailscale device request returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Devices []device `json:"devices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 10<<20)).Decode(&payload); err != nil {
		return domain.Snapshot{}, fmt.Errorf("decode Tailscale devices: %w", err)
	}
	if payload.Devices == nil {
		return domain.Snapshot{}, errors.New("Tailscale response omitted devices")
	}
	snapshot := domain.Snapshot{Nodes: make([]domain.Node, 0, len(payload.Devices)), Services: []domain.Service{}, Activity: []domain.Activity{}, Connections: []domain.Connection{}}
	seen := make(map[string]bool, len(payload.Devices))
	for _, item := range payload.Devices {
		if item.ID == "" || seen[item.ID] {
			return domain.Snapshot{}, errors.New("Tailscale response has a missing or duplicate device ID")
		}
		seen[item.ID] = true
		name := item.Hostname
		if name == "" {
			name = strings.TrimSuffix(item.Name, ".")
		}
		if name == "" {
			name = "Tailscale device " + item.ID
		}
		platform := item.OS
		if platform == "" {
			platform = "Not reported"
		}
		var lastSeen *time.Time
		if item.LastSeen != "" {
			parsed, err := time.Parse(time.RFC3339Nano, item.LastSeen)
			if err != nil {
				return domain.Snapshot{}, fmt.Errorf("invalid lastSeen for Tailscale device %s: %w", item.ID, err)
			}
			utc := parsed.UTC()
			lastSeen = &utc
		}
		health := domain.Health{State: "unknown", ObservedAt: now.UTC(), StaleAfterSeconds: 180, Source: "tailscale"}
		snapshot.Nodes = append(snapshot.Nodes, domain.Node{
			ID: "tailscale-" + item.ID, Name: name, Role: "Tailnet device", Platform: platform,
			Health: health, Freshness: health.Freshness(now), Capabilities: []string{}, Metrics: domain.Metrics{},
			Addresses: append([]string{}, item.Addresses...), LastSeen: lastSeen, Simulated: false,
		})
	}
	return snapshot, nil
}

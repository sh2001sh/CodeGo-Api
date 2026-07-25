package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
	"golang.org/x/sync/errgroup"
)

const (
	requestTimeout   = 30 * time.Second
	httpTimeout      = 10 * time.Second
	uptimeKeySuffix  = "_24"
	apiStatusPath    = "/api/status-page/"
	apiHeartbeatPath = "/api/status-page/heartbeat/"
)

// UptimeMonitor mirrors a single public Uptime Kuma monitor summary.
type UptimeMonitor struct {
	Name   string  `json:"name"`
	Uptime float64 `json:"uptime"`
	Status int     `json:"status"`
	Group  string  `json:"group,omitempty"`
}

// UptimeGroupResult mirrors the public grouped uptime payload.
type UptimeGroupResult struct {
	CategoryName string          `json:"categoryName"`
	Monitors     []UptimeMonitor `json:"monitors"`
}

// GetUptimeStatus fetches grouped Uptime Kuma public status payloads.
func GetUptimeStatus(ctx context.Context) []UptimeGroupResult {
	groups := platformstore.GetUptimeKumaGroups()
	if len(groups) == 0 {
		return []UptimeGroupResult{}
	}

	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	client := &http.Client{Timeout: httpTimeout}
	results := make([]UptimeGroupResult, len(groups))

	g, groupCtx := errgroup.WithContext(requestCtx)
	for i, group := range groups {
		i := i
		group := group
		g.Go(func() error {
			results[i] = fetchUptimeGroupData(groupCtx, client, group)
			return nil
		})
	}

	_ = g.Wait()
	return results
}

func fetchUptimeGroupData(ctx context.Context, client *http.Client, groupConfig map[string]any) UptimeGroupResult {
	url, _ := groupConfig["url"].(string)
	slug, _ := groupConfig["slug"].(string)
	categoryName, _ := groupConfig["categoryName"].(string)

	result := UptimeGroupResult{
		CategoryName: categoryName,
		Monitors:     []UptimeMonitor{},
	}
	if url == "" || slug == "" {
		return result
	}

	baseURL := strings.TrimSuffix(url, "/")
	var statusData struct {
		PublicGroupList []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			MonitorList []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"monitorList"`
		} `json:"publicGroupList"`
	}
	var heartbeatData struct {
		HeartbeatList map[string][]struct {
			Status int `json:"status"`
		} `json:"heartbeatList"`
		UptimeList map[string]float64 `json:"uptimeList"`
	}

	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return getAndDecode(groupCtx, client, baseURL+apiStatusPath+slug, &statusData)
	})
	g.Go(func() error {
		return getAndDecode(groupCtx, client, baseURL+apiHeartbeatPath+slug, &heartbeatData)
	})
	if g.Wait() != nil {
		return result
	}

	for _, publicGroup := range statusData.PublicGroupList {
		for _, monitor := range publicGroup.MonitorList {
			item := UptimeMonitor{
				Name:  monitor.Name,
				Group: publicGroup.Name,
			}
			monitorID := strconv.Itoa(monitor.ID)
			if uptime, exists := heartbeatData.UptimeList[monitorID+uptimeKeySuffix]; exists {
				item.Uptime = uptime
			}
			if heartbeats, exists := heartbeatData.HeartbeatList[monitorID]; exists && len(heartbeats) > 0 {
				item.Status = heartbeats[0].Status
			}
			result.Monitors = append(result.Monitors, item)
		}
	}

	return result
}

func getAndDecode(ctx context.Context, client *http.Client, requestURL string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("non-200 status")
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

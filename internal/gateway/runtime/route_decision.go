package runtime

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
)

const routeDecisionContextKey = "route_decision_audit"

// RouteDecision is an internal-only record attached to the existing request audit log.
// It contains identifiers, not provider credentials or channel names.
type RouteDecision struct {
	RequestID       string   `json:"request_id"`
	Model           string   `json:"model"`
	RequestedGroup  string   `json:"requested_group"`
	SelectedGroup   string   `json:"selected_group,omitempty"`
	Mode            string   `json:"mode,omitempty"`
	ChannelID       int      `json:"channel_id,omitempty"`
	CandidateGroups int      `json:"candidate_groups"`
	Excluded        []string `json:"excluded,omitempty"`
	RetryCount      int      `json:"retry_count"`
	AffinityHit     bool     `json:"affinity_hit"`
	Fallback        bool     `json:"fallback"`
	HealthState     string   `json:"health_state,omitempty"`
}

// MarkAutomaticPool records that the selected channel came from the cost and
// health based automatic pool rather than legacy priority/weight selection.
func MarkAutomaticPool(c *gin.Context) {
	updateRouteDecision(c, func(decision *RouteDecision) {
		decision.Mode = "automatic_pool"
	})
}

func StartRouteDecision(c *gin.Context, model string, requestedGroup string) {
	if c == nil {
		return
	}
	c.Set(routeDecisionContextKey, RouteDecision{
		RequestID:      c.GetString(constant.RequestIdKey),
		Model:          strings.TrimSpace(model),
		RequestedGroup: strings.TrimSpace(requestedGroup),
	})
}

func UpdateRouteDecisionCandidates(c *gin.Context, count int) {
	updateRouteDecision(c, func(decision *RouteDecision) {
		if count > decision.CandidateGroups {
			decision.CandidateGroups = count
		}
	})
}

func ExcludeRouteDecisionCandidate(c *gin.Context, reason string) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return
	}
	updateRouteDecision(c, func(decision *RouteDecision) {
		for _, existing := range decision.Excluded {
			if existing == reason {
				return
			}
		}
		decision.Excluded = append(decision.Excluded, reason)
	})
}

func SelectRouteDecisionCandidate(c *gin.Context, group string, channelID int, affinityHit bool) {
	updateRouteDecision(c, func(decision *RouteDecision) {
		if decision.SelectedGroup != "" && decision.SelectedGroup != group {
			decision.Fallback = true
		}
		decision.SelectedGroup = strings.TrimSpace(group)
		decision.ChannelID = channelID
		decision.AffinityHit = affinityHit
		if health, found := GetChannelHealth(channelID, decision.Model); found {
			decision.HealthState = health.State
		} else {
			decision.HealthState = ChannelHealthHealthy
		}
	})
}

func RecordRouteDecisionRetry(c *gin.Context) {
	updateRouteDecision(c, func(decision *RouteDecision) {
		decision.RetryCount++
		decision.Fallback = true
	})
}

// GetRouteDecision returns a copy suitable for administrators' log metadata.
func GetRouteDecision(c *gin.Context) (RouteDecision, bool) {
	if c == nil {
		return RouteDecision{}, false
	}
	value, ok := c.Get(routeDecisionContextKey)
	if !ok {
		return RouteDecision{}, false
	}
	decision, ok := value.(RouteDecision)
	return decision, ok
}

func updateRouteDecision(c *gin.Context, update func(*RouteDecision)) {
	if c == nil {
		return
	}
	value, ok := c.Get(routeDecisionContextKey)
	if !ok {
		return
	}
	decision, ok := value.(RouteDecision)
	if !ok {
		return
	}
	update(&decision)
	c.Set(routeDecisionContextKey, decision)
}

package app

import (
	"encoding/json"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayruntime "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
)

const (
	routePoolExploreRate        = 0.025
	routePoolProbeRate          = 0.02
	routePoolAffinityContextKey = "automatic_route_pool_affinity"
	routePoolAffinityTTL        = 3 * time.Minute
	routePoolSwitchImprovement  = 0.15
)

type scoredRoutePoolCandidate struct {
	channel *gatewayschema.Channel
	score   float64
	probe   bool
}

type routePoolAffinity struct {
	CacheKey string
}

// selectAutomaticPoolChannel returns managed=true when the group has an enabled
// automatic pool. In that case priority and weight are deliberately ignored.
func selectAutomaticPoolChannel(c *gin.Context, group, modelName string) (*gatewayschema.Channel, bool, error) {
	detail, err := gatewaystore.LoadEnabledRoutePool(group)
	if err != nil || detail == nil {
		return nil, detail != nil, err
	}
	candidates, err := gatewaystore.LoadRoutePoolCandidates(group, modelName, detail)
	if err != nil {
		return nil, true, err
	}
	now := time.Now()
	healthy := make([]scoredRoutePoolCandidate, 0, len(candidates))
	probes := make([]scoredRoutePoolCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if channelAlreadyUsed(c, candidate.Channel.Id) {
			continue
		}
		health, found := gatewayruntime.GetChannelHealth(candidate.Channel.Id, modelName)
		if found && health.State == gatewayruntime.ChannelHealthCooling {
			if health.CoolingUntil.After(now) {
				continue
			}
			probes = append(probes, scoredRoutePoolCandidate{channel: candidate.Channel, score: effectiveRoutePoolCost(candidate.Member, modelName, health), probe: true})
			continue
		}
		healthy = append(healthy, scoredRoutePoolCandidate{channel: candidate.Channel, score: effectiveRoutePoolCost(candidate.Member, modelName, health)})
	}

	applyRoutePoolTTFTPenalty(healthy, modelName)
	applyRoutePoolTTFTPenalty(probes, modelName)
	prepareRoutePoolAffinity(c, detail.Pool.ID, group, modelName)
	if len(healthy) == 0 {
		return selectRoutePoolCandidate(c, chooseLowestRoutePoolCandidate(probes)), true, nil
	}
	if sticky := getRoutePoolStickyCandidate(c, healthy, modelName); sticky != nil {
		return selectRoutePoolCandidate(c, sticky), true, nil
	}
	if len(probes) > 0 && rand.Float64() < routePoolProbeRate {
		return selectRoutePoolCandidate(c, chooseLowestRoutePoolCandidate(probes)), true, nil
	}
	return selectRoutePoolCandidate(c, chooseRoutePoolHealthyCandidate(healthy)), true, nil
}

// RecordAutomaticPoolAffinity keeps an unbound token on the selected pool
// member for a short period. Explicit cache affinity remains independent.
func RecordAutomaticPoolAffinity(c *gin.Context, selectedChannelID int) {
	if c == nil {
		return
	}
	affinity, ok := c.Get(routePoolAffinityContextKey)
	if !ok {
		return
	}
	value, ok := affinity.(routePoolAffinity)
	if !ok || value.CacheKey == "" {
		return
	}
	if successfulChannelID := c.GetInt(string(constant.ContextKeyChannelId)); successfulChannelID > 0 {
		selectedChannelID = successfulChannelID
	}
	if selectedChannelID > 0 {
		_ = gatewayruntime.RecordPreferredChannel(value.CacheKey, selectedChannelID, int(routePoolAffinityTTL.Seconds()))
	}
}

// ShouldMigrateAutomaticPoolAffinity permits explicit cache affinity to escape
// an unhealthy automatic-pool member without making healthy sessions drift.
func ShouldMigrateAutomaticPoolAffinity(group, modelName string, channelID int) bool {
	detail, err := gatewaystore.LoadEnabledRoutePool(group)
	if err != nil || detail == nil || channelID <= 0 {
		return false
	}
	candidates, err := gatewaystore.LoadRoutePoolCandidates(group, modelName, detail)
	if err != nil {
		return false
	}
	now := time.Now()
	healthy := make([]scoredRoutePoolCandidate, 0, len(candidates))
	var current *scoredRoutePoolCandidate
	for _, candidate := range candidates {
		health, found := gatewayruntime.GetChannelHealth(candidate.Channel.Id, modelName)
		if found && health.State == gatewayruntime.ChannelHealthCooling && health.CoolingUntil.After(now) {
			continue
		}
		scored := scoredRoutePoolCandidate{
			channel: candidate.Channel,
			score:   effectiveRoutePoolCost(candidate.Member, modelName, health),
		}
		healthy = append(healthy, scored)
		if candidate.Channel.Id == channelID {
			current = &healthy[len(healthy)-1]
		}
	}
	if current == nil || len(healthy) < 2 {
		return false
	}
	applyRoutePoolTTFTPenalty(healthy, modelName)
	for index := range healthy {
		if healthy[index].channel.Id == channelID {
			current = &healthy[index]
			break
		}
	}
	best := chooseLowestRoutePoolCandidate(healthy)
	if best == nil || best.channel.Id == channelID {
		return false
	}
	health, _ := gatewayruntime.GetChannelHealth(channelID, modelName)
	if routePoolHardMigrationRequired(health, routePoolMedianTTFT(healthy, modelName)) {
		return true
	}
	return (health.State == gatewayruntime.ChannelHealthDegraded || routePoolReliabilityNeedsMigration(health)) &&
		best.score <= current.score*(1-routePoolSwitchImprovement)
}

func selectRoutePoolCandidate(c *gin.Context, candidate *scoredRoutePoolCandidate) *gatewayschema.Channel {
	if candidate == nil {
		return nil
	}
	if c != nil {
		gatewayruntime.MarkAutomaticPool(c)
	}
	return candidate.channel
}

func chooseRoutePoolHealthyCandidate(candidates []scoredRoutePoolCandidate) *scoredRoutePoolCandidate {
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score < candidates[j].score })
	best := candidates[0]
	if len(candidates) == 1 || rand.Float64() >= routePoolExploreRate {
		return &best
	}
	limit := best.score * 1.15
	explorable := make([]scoredRoutePoolCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.score <= limit {
			explorable = append(explorable, candidate)
		}
	}
	if len(explorable) == 0 {
		return &best
	}
	selected := explorable[rand.Intn(len(explorable))]
	return &selected
}

func prepareRoutePoolAffinity(c *gin.Context, poolID int64, group, modelName string) {
	if c == nil || poolID <= 0 || c.GetInt(string(constant.ContextKeyTokenId)) <= 0 {
		return
	}
	key := strings.Join([]string{
		"route_pool",
		strconv.FormatInt(poolID, 10),
		strconv.Itoa(c.GetInt(string(constant.ContextKeyTokenId))),
		group,
		modelName,
	}, ":")
	c.Set(routePoolAffinityContextKey, routePoolAffinity{CacheKey: key})
}

func getRoutePoolStickyCandidate(c *gin.Context, candidates []scoredRoutePoolCandidate, modelName string) *scoredRoutePoolCandidate {
	if c == nil {
		return nil
	}
	value, ok := c.Get(routePoolAffinityContextKey)
	if !ok {
		return nil
	}
	affinity, ok := value.(routePoolAffinity)
	if !ok || affinity.CacheKey == "" {
		return nil
	}
	channelID, found, err := gatewayruntime.GetPreferredChannel(affinity.CacheKey)
	if err != nil || !found {
		return nil
	}
	var sticky *scoredRoutePoolCandidate
	for index := range candidates {
		if candidates[index].channel.Id == channelID {
			sticky = &candidates[index]
			break
		}
	}
	if sticky == nil || channelAlreadyUsed(c, channelID) {
		gatewayruntime.InvalidatePreferredChannel(affinity.CacheKey)
		return nil
	}
	health, _ := gatewayruntime.GetChannelHealth(channelID, modelName)
	if routePoolHardMigrationRequired(health, routePoolMedianTTFT(candidates, modelName)) {
		gatewayruntime.InvalidatePreferredChannel(affinity.CacheKey)
		return nil
	}
	best := chooseLowestRoutePoolCandidate(candidates)
	if best != nil && best.channel.Id != channelID && best.score <= sticky.score*(1-routePoolSwitchImprovement) {
		return nil
	}
	return sticky
}

func chooseLowestRoutePoolCandidate(candidates []scoredRoutePoolCandidate) *scoredRoutePoolCandidate {
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score < candidates[j].score })
	return &candidates[0]
}

func effectiveRoutePoolCost(member gatewayschema.RoutePoolMember, modelName string, health gatewayruntime.ChannelHealth) float64 {
	cost := routePoolModelCost(member, modelName)
	if cost <= 0 {
		cost = 1
	}
	if health.Window5Requests < 20 {
		cost *= 1.10
	}
	if health.Window5Requests >= 5 {
		cost *= routePoolReliabilityPenalty(routePoolConservativeSuccessRate(health))
	}
	if health.State == gatewayruntime.ChannelHealthDegraded {
		cost *= 1.35
	}
	if health.ConsecutiveRetryableFailures > 0 {
		cost *= math.Pow(1.25, float64(health.ConsecutiveRetryableFailures))
	}
	return cost
}

func routePoolReliabilityPenalty(rate float64) float64 {
	switch {
	case rate >= 98:
		return 1
	case rate >= 95:
		return 1.15 + (98-rate)*0.12
	case rate >= 90:
		return 2.5 + (95-rate)*0.3
	default:
		return math.Min(15, 5+(90-rate)*0.2)
	}
}

func routePoolConservativeSuccessRate(health gatewayruntime.ChannelHealth) float64 {
	requests := health.Window5Requests
	successes := health.Window5Successes
	if requests < 20 || successes < 0 || successes > requests {
		return health.SuccessRate5m
	}
	p := float64(successes) / float64(requests)
	z := 1.96
	denominator := 1 + z*z/float64(requests)
	center := p + z*z/(2*float64(requests))
	margin := z * math.Sqrt((p*(1-p)+z*z/(4*float64(requests)))/float64(requests))
	lowerBound := (center - margin) / denominator * 100
	return health.SuccessRate5m*0.8 + lowerBound*0.2
}

func routePoolModelCost(member gatewayschema.RoutePoolMember, modelName string) float64 {
	cost := member.CostMultiplier
	var overrides map[string]float64
	if err := json.Unmarshal([]byte(member.ModelCostOverrides), &overrides); err == nil {
		if override, ok := overrides[modelName]; ok && override > 0 {
			cost = override
		}
	}
	return cost
}

func applyRoutePoolTTFTPenalty(candidates []scoredRoutePoolCandidate, modelName string) {
	if len(candidates) < 2 {
		return
	}
	values := make([]float64, 0, len(candidates))
	for _, candidate := range candidates {
		health, found := gatewayruntime.GetChannelHealth(candidate.channel.Id, modelName)
		if found && health.TTFTP95Milliseconds > 0 {
			values = append(values, health.TTFTP95Milliseconds)
		}
	}
	if len(values) == 0 {
		return
	}
	sort.Float64s(values)
	median := values[(len(values)-1)/2]
	if median <= 0 {
		return
	}
	for index := range candidates {
		health, found := gatewayruntime.GetChannelHealth(candidates[index].channel.Id, modelName)
		if !found || health.TTFTP95Milliseconds <= 0 {
			continue
		}
		ratio := health.TTFTP95Milliseconds / median
		switch {
		case ratio > 2.5:
			candidates[index].score *= 2
		case ratio > 1.5:
			candidates[index].score *= 1.35
		}
	}
}

func routePoolMedianTTFT(candidates []scoredRoutePoolCandidate, modelName string) float64 {
	values := make([]float64, 0, len(candidates))
	for _, candidate := range candidates {
		health, found := gatewayruntime.GetChannelHealth(candidate.channel.Id, modelName)
		if found && health.TTFTP95Milliseconds > 0 {
			values = append(values, health.TTFTP95Milliseconds)
		}
	}
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	return values[(len(values)-1)/2]
}

func routePoolHardMigrationRequired(health gatewayruntime.ChannelHealth, medianTTFT float64) bool {
	if health.State == gatewayruntime.ChannelHealthCooling && health.CoolingUntil.After(time.Now()) {
		return true
	}
	if health.ConsecutiveRetryableFailures >= 2 {
		return true
	}
	if health.Window5Requests >= 10 && routePoolConservativeSuccessRate(health) < 85 {
		return true
	}
	return medianTTFT > 0 && health.TTFTP95Milliseconds > medianTTFT*2.5
}

func routePoolReliabilityNeedsMigration(health gatewayruntime.ChannelHealth) bool {
	return health.Window5Requests >= 20 && health.SuccessRate5m < 95
}

func channelAlreadyUsed(c *gin.Context, channelID int) bool {
	if c == nil || channelID <= 0 {
		return false
	}
	needle := strconv.Itoa(channelID)
	for _, used := range c.GetStringSlice("use_channel") {
		if strings.TrimSpace(used) == needle {
			return true
		}
	}
	return false
}

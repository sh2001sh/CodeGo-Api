package temporal

import (
	"fmt"
	"os"
	"strings"

	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/activities"
	temporalclient "go.temporal.io/sdk/client"
)

const (
	defaultTemporalNamespace = "default"
	defaultTaskQueueTasks    = "workflow-tasks"
	defaultTaskQueueBilling  = "workflow-billing"
	defaultTaskQueueOrders   = "workflow-orders"
	defaultTaskQueueSubs     = "workflow-subscriptions"
)

type Config struct {
	HostPort               string
	Namespace              string
	TaskQueueTasks         string
	TaskQueueBilling       string
	TaskQueueOrders        string
	TaskQueueSubscriptions string
}

type WorkerDependencies struct {
	TaskActivities    *activities.TaskActivities
	BillingActivities *activities.BillingActivities
	GatewayActivities *activities.GatewayActivities
	OrderActivities   *activities.OrderActivities
}

// LoadConfigFromEnv resolves the Temporal worker configuration for workflow-worker.
func LoadConfigFromEnv() Config {
	return Config{
		HostPort:               getenvTrimmed("TEMPORAL_HOSTPORT"),
		Namespace:              defaultIfEmpty(getenvTrimmed("TEMPORAL_NAMESPACE"), defaultTemporalNamespace),
		TaskQueueTasks:         defaultIfEmpty(getenvTrimmed("TEMPORAL_TASK_QUEUE_TASKS"), defaultTaskQueueTasks),
		TaskQueueBilling:       defaultIfEmpty(getenvTrimmed("TEMPORAL_TASK_QUEUE_BILLING"), defaultTaskQueueBilling),
		TaskQueueOrders:        defaultIfEmpty(getenvTrimmed("TEMPORAL_TASK_QUEUE_ORDERS"), defaultTaskQueueOrders),
		TaskQueueSubscriptions: defaultIfEmpty(getenvTrimmed("TEMPORAL_TASK_QUEUE_SUBSCRIPTIONS"), defaultTaskQueueSubs),
	}
}

// Validate checks whether the enabled Temporal worker has enough runtime config to start.
func (c Config) Validate() error {
	if c.HostPort == "" {
		return fmt.Errorf("TEMPORAL_HOSTPORT is required for workflow-worker")
	}
	return nil
}

// NewDefaultWorkerDependencies creates the first-cut activity set for the Temporal worker.
func NewDefaultWorkerDependencies() WorkerDependencies {
	return WorkerDependencies{
		TaskActivities:    &activities.TaskActivities{},
		BillingActivities: &activities.BillingActivities{},
		GatewayActivities: &activities.GatewayActivities{},
		OrderActivities:   &activities.OrderActivities{},
	}
}

// RunWorker dials Temporal and starts all configured workflow workers until the process receives a stop signal.
func RunWorker(cfg Config, deps WorkerDependencies) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	client, err := temporalclient.Dial(temporalclient.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return err
	}
	defer client.Close()

	registry := NewWorkerRegistry(client, cfg)
	bootstrap := WorkerBootstrap{
		Client: client,
		Deps:   deps,
	}
	return registry.Run(&bootstrap)
}

func defaultIfEmpty(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func getenvTrimmed(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

package temporal

import "testing"

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("TEMPORAL_HOSTPORT", "")
	t.Setenv("TEMPORAL_NAMESPACE", "")
	t.Setenv("TEMPORAL_TASK_QUEUE_TASKS", "")
	t.Setenv("TEMPORAL_TASK_QUEUE_BILLING", "")
	t.Setenv("TEMPORAL_TASK_QUEUE_ORDERS", "")
	t.Setenv("TEMPORAL_TASK_QUEUE_SUBSCRIPTIONS", "")

	cfg := LoadConfigFromEnv()
	if cfg.HostPort != "" {
		t.Fatalf("unexpected host port: %q", cfg.HostPort)
	}
	if cfg.Namespace != defaultTemporalNamespace {
		t.Fatalf("unexpected namespace: %s", cfg.Namespace)
	}
	if cfg.TaskQueueTasks != defaultTaskQueueTasks {
		t.Fatalf("unexpected task queue: %s", cfg.TaskQueueTasks)
	}
	if cfg.TaskQueueBilling != defaultTaskQueueBilling {
		t.Fatalf("unexpected billing queue: %s", cfg.TaskQueueBilling)
	}
	if cfg.TaskQueueOrders != defaultTaskQueueOrders {
		t.Fatalf("unexpected orders queue: %s", cfg.TaskQueueOrders)
	}
	if cfg.TaskQueueSubscriptions != defaultTaskQueueSubs {
		t.Fatalf("unexpected subscriptions queue: %s", cfg.TaskQueueSubscriptions)
	}
}

func TestConfigValidateRequiresHostPortWhenEnabled(t *testing.T) {
	cfg := Config{}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected host port validation error")
	}
}

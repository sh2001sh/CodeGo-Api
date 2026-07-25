package temporal

import (
	"testing"

	temporalactivity "go.temporal.io/sdk/activity"
	temporalworkflow "go.temporal.io/sdk/workflow"
)

type fakeRegistrar struct {
	workflows  []string
	activities []string
}

func (f *fakeRegistrar) RegisterWorkflowWithOptions(_ interface{}, options temporalworkflow.RegisterOptions) {
	f.workflows = append(f.workflows, options.Name)
}

func (f *fakeRegistrar) RegisterActivityWithOptions(_ interface{}, options temporalactivity.RegisterOptions) {
	f.activities = append(f.activities, options.Name)
}

func TestRegisterTaskWorker(t *testing.T) {
	bootstrap := &WorkerBootstrap{Deps: NewDefaultWorkerDependencies()}
	fake := &fakeRegistrar{}
	registerTaskWorker(bootstrap, fake)

	if len(fake.workflows) != 1 {
		t.Fatalf("expected 1 task workflow, got %d", len(fake.workflows))
	}
	if len(fake.activities) != 7 {
		t.Fatalf("expected 7 task activities, got %d", len(fake.activities))
	}
}

func TestRegisterBillingWorker(t *testing.T) {
	bootstrap := &WorkerBootstrap{Deps: NewDefaultWorkerDependencies()}
	fake := &fakeRegistrar{}
	registerBillingWorker(bootstrap, fake)

	if len(fake.workflows) != 1 {
		t.Fatalf("expected 1 billing workflow, got %d", len(fake.workflows))
	}
	if len(fake.activities) != 7 {
		t.Fatalf("expected 7 billing activities, got %d", len(fake.activities))
	}
}

func TestRegisterOrderWorker(t *testing.T) {
	bootstrap := &WorkerBootstrap{Deps: NewDefaultWorkerDependencies()}
	fake := &fakeRegistrar{}
	registerOrderWorker(bootstrap, fake)

	if len(fake.workflows) != 1 {
		t.Fatalf("expected 1 order workflow, got %d", len(fake.workflows))
	}
	if len(fake.activities) != 5 {
		t.Fatalf("expected 5 order activities, got %d", len(fake.activities))
	}
}

func TestRegisterSubscriptionWorker(t *testing.T) {
	bootstrap := &WorkerBootstrap{Deps: NewDefaultWorkerDependencies()}
	fake := &fakeRegistrar{}
	registerSubscriptionWorker(bootstrap, fake)

	if len(fake.workflows) != 1 {
		t.Fatalf("expected 1 subscription workflow, got %d", len(fake.workflows))
	}
	if len(fake.activities) != 4 {
		t.Fatalf("expected 4 subscription activities, got %d", len(fake.activities))
	}
}

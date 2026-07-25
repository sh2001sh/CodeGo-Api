package app

import (
	"testing"

	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	"github.com/stretchr/testify/assert"
)

func TestSubscriptionResetWorkflowIDIncludesDueTimestamp(t *testing.T) {
	subscription := commerceschema.UserSubscription{Id: 42, NextResetTime: 1_752_296_400}

	assert.Equal(t, "subscription-reset-42-1752296400", subscriptionResetWorkflowID(subscription))
}

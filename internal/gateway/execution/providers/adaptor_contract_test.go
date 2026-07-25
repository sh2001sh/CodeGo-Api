package providers

import (
	"fmt"
	"testing"

	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/stretchr/testify/require"
)

func TestSyncAdaptorFactoryProvidesContractForSupportedAPITypes(t *testing.T) {
	apiTypes := []int{
		constant.APITypeOpenAI,
		constant.APITypeAnthropic,
		constant.APITypeGemini,
		constant.APITypeAws,
		constant.APITypeAli,
		constant.APITypeBaidu,
		constant.APITypeCohere,
		constant.APITypeCodex,
	}

	for _, apiType := range apiTypes {
		t.Run(fmt.Sprintf("api_type_%d", apiType), func(t *testing.T) {
			require.NotNil(t, NewSyncAdaptor(apiType))
		})
	}
}

package ali

import (
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/dto"
)

const EnableSearchModelSuffix = "-internet"

func requestOpenAI2Ali(request dto.GeneralOpenAIRequest) *dto.GeneralOpenAIRequest {
	topP := lo.FromPtrOr(request.TopP, 0)
	if topP >= 1 {
		request.TopP = lo.ToPtr(0.999)
	} else if topP <= 0 {
		request.TopP = lo.ToPtr(0.001)
	}
	return &request
}

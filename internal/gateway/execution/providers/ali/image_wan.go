package ali

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
)

func oaiFormEdit2WanxImageEdit(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (*AliImageRequest, error) {
	var imageRequest AliImageRequest
	imageRequest.Model = request.Model
	imageRequest.ResponseFormat = request.ResponseFormat

	wanInput := WanImageInput{Prompt: request.Prompt}
	if err := platformhttpx.UnmarshalBodyReusable(c, &wanInput); err != nil {
		return nil, err
	}
	var err error
	if wanInput.Images, err = getImageBase64sFromForm(c, "image"); err != nil {
		return nil, fmt.Errorf("get image base64s from form failed: %w", err)
	}

	imageRequest.Input = wanInput
	imageRequest.Parameters = AliImageParameters{
		N: int(lo.FromPtrOr(request.N, uint(1))),
	}
	info.PriceData.AddOtherRatio("n", float64(imageRequest.Parameters.N))
	return &imageRequest, nil
}

func isOldWanModel(modelName string) bool {
	return strings.Contains(modelName, "wan") &&
		!lo.SomeBy([]string{"wan2.6", "wan2.7"}, func(v string) bool { return strings.Contains(modelName, v) })
}

func isWanModel(modelName string) bool {
	return strings.Contains(modelName, "wan")
}

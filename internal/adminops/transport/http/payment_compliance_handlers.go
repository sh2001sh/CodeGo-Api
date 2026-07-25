package http

import (
	"github.com/gin-gonic/gin"
	adminopsapp "github.com/sh2001sh/CodeGo-Api/internal/adminops/app"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
)

type paymentComplianceRequest struct {
	Confirmed bool `json:"confirmed"`
}

// ConfirmPaymentCompliance confirms payment compliance using dashboard session auth.
func ConfirmPaymentCompliance(c *gin.Context) {
	if c.GetBool("use_access_token") {
		c.JSON(stdhttp.StatusForbidden, gin.H{
			"success": false,
			"message": "This operation requires dashboard session authentication. API access token is not allowed.",
		})
		return
	}

	var req paymentComplianceRequest
	if err := platformencoding.DecodeJSON(c.Request.Body, &req); err != nil {
		httpapi.ApiErrorMsg(c, "参数错误")
		return
	}
	if !req.Confirmed {
		httpapi.ApiErrorMsg(c, adminopsapp.ErrPaymentComplianceConfirmationRequired.Error())
		return
	}

	result, err := adminopsapp.ConfirmPaymentCompliance(c.Request.Context(), c.GetInt("id"), c.ClientIP())
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}
	httpapi.ApiSuccess(c, result)
}

package http

import (
	"errors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	"net/http"
)

func BindEmail(c *gin.Context) {
	var req identityapp.EmailBindRequest
	if err := platformencoding.DecodeJSON(c.Request.Body, &req); err != nil {
		httpapi.ApiError(c, identityapp.ErrInvalidRequestBody)
		return
	}

	session := sessions.Default(c)
	idRaw := session.Get("id")
	id, ok := idRaw.(int)
	if !ok || id == 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgUnauthorized)
		return
	}

	if err := identityapp.BindEmail(id, req.Email, req.Code); err != nil {
		switch {
		case errors.Is(err, identityapp.ErrVerificationCodeInvalid):
			httpapi.ApiErrorI18n(c, i18n.MsgUserVerificationCodeError)
		default:
			httpapi.ApiError(c, err)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

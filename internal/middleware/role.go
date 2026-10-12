package middleware

import (
	"github.com/gin-gonic/gin"

	"resdownload.com/backend/internal/model"
	"resdownload.com/backend/internal/pkg/response"
)

// RequireRole rejects authenticated callers whose role differs. It must run
// after Auth; without an authenticated role it answers 401, with a wrong
// role 403.
func RequireRole(role model.Role) gin.HandlerFunc {
	want := string(role)
	return func(c *gin.Context) {
		got := RoleFromContext(c)
		if got == "" {
			response.Fail(c, response.ErrUnauthorized)
			return
		}
		if got != want {
			response.Fail(c, response.ErrForbidden)
			return
		}
		c.Next()
	}
}

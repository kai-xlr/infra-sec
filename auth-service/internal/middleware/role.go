package middleware

import (
	"net/http"

	"auth-service/internal/audit"
	"auth-service/internal/models"
)

func RequireRole(role string, auditLogger *audit.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claimsValue := r.Context().Value(models.ClaimsKey)

			claims, ok := claimsValue.(*models.CustomClaims)
			if !ok || claims == nil {
				auditLogger.Log(audit.Event{
					User:     "UNKNOWN",
					Role:     "UNKNOWN",
					Action:   "ACCESS_ROLE_" + role,
					Resource: r.URL.Path,
					Result:   "deny",
				})

				jsonError(
					w,
					http.StatusText(http.StatusUnauthorized),
					http.StatusUnauthorized,
				)
				return
			}

			allowed := claims.Role == role

			result := "deny"
			if allowed {
				result = "allow"
			}

			auditLogger.Log(audit.Event{
				User:     claims.Subject,
				Role:     claims.Role,
				Action:   "ACCESS_ROLE_" + role,
				Resource: r.URL.Path,
				Result:   result,
			})

			if !allowed {
				jsonError(
					w,
					http.StatusText(http.StatusForbidden),
					http.StatusForbidden,
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

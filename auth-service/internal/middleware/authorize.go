package middleware

import (
	"log"
	"net/http"

	"auth-service/internal/models"
	"auth-service/internal/rbac"
)

func RequirePermission(action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claimsValue := r.Context().Value(models.ClaimsKey)

			claims, ok := claimsValue.(*models.CustomClaims)
			if !ok || claims == nil {
				log.Printf(
					"role=%s action=%s decision=%s",
					"UNKNOWN",
					action,
					"DENY",
				)

				http.Error(
					w,
					http.StatusText(http.StatusUnauthorized),
					http.StatusUnauthorized,
				)
				return
			}

			allowed := rbac.Authorize(claims.Role, action)

			decision := "DENY"
			if allowed {
				decision = "ALLOW"
			}

			log.Printf(
				"role=%s action=%s decision=%s",
				claims.Role,
				action,
				decision,
			)

			if !allowed {
				http.Error(
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

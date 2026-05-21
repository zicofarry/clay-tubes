package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zicofarry/clay-app/backend/services/gateway/config"
)

// ClayContext holds the claims extracted from a verified JWT.
// Attached to the request context by Auth middleware.
type ClayContext struct {
	UserID string
	Role   string
	JTI    string
}

type clayContextKey struct{}

// GetClayContext retrieves ClayContext from the request context.
// Returns nil if auth middleware was not applied (e.g. public route).
func GetClayContext(ctx context.Context) *ClayContext {
	v, _ := ctx.Value(clayContextKey{}).(*ClayContext)
	return v
}

// clayClaims maps the JWT payload fields used by Clay.
type clayClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Auth returns a middleware that validates JWT tokens based on the given AuthRule.
//
// Behaviour:
//   - rule.Required == false → skip JWT validation entirely (public route)
//   - rule.Required == true  → JWT must be present and valid
//   - rule.Roles non-empty   → JWT role must match one of the allowed roles
//
// On success, injects ClayContext into request context and sets:
//   - X-User-ID  header (for downstream services)
//   - X-User-Role header (for downstream services)
func Auth(secret string, issuer string, rule config.AuthRule) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rule.Required {
				next.ServeHTTP(w, r)
				return
			}

			// Extract Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeAuthError(w, http.StatusUnauthorized, "MISSING_TOKEN", "Authorization header is required")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeAuthError(w, http.StatusUnauthorized, "INVALID_TOKEN_FORMAT", "Authorization header must be: Bearer <token>")
				return
			}
			tokenString := parts[1]

			// Parse and validate JWT
			claims, err := parseJWT(tokenString, secret, issuer)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
				return
			}

			// Role check
			if len(rule.Roles) > 0 && !containsRole(rule.Roles, claims.Role) {
				writeAuthError(w, http.StatusForbidden, "FORBIDDEN",
					fmt.Sprintf("role '%s' is not allowed for this endpoint", claims.Role))
				return
			}

			// Attach context and inject downstream headers
			cc := &ClayContext{
				UserID: claims.UserID,
				Role:   claims.Role,
				JTI:    claims.ID,
			}
			ctx := context.WithValue(r.Context(), clayContextKey{}, cc)

			// These headers are read by downstream services via clay-shared AuthContext middleware
			r = r.WithContext(ctx)
			r.Header.Set("X-User-ID", claims.UserID)
			r.Header.Set("X-User-Role", claims.Role)

			next.ServeHTTP(w, r)
		})
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func parseJWT(tokenString, secret, issuer string) (*clayClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&clayClaims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secret), nil
		},
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	claims, ok := token.Claims.(*clayClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.UserID == "" {
		return nil, fmt.Errorf("token missing user_id claim")
	}

	return claims, nil
}

func containsRole(allowed []string, role string) bool {
	for _, r := range allowed {
		if r == role {
			return true
		}
	}
	return false
}

func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"code":%q,"message":%q}`, code, message)
}

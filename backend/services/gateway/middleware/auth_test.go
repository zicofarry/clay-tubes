//go:build unit

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zicofarry/clay-app/backend/services/gateway/config"
)

const testSecret = "test-secret-key-for-unit-tests-only"
const testIssuer = "clay-auth-service"

func makeToken(t *testing.T, userID, role string, expired bool) string {
	t.Helper()
	exp := time.Now().Add(15 * time.Minute)
	if expired {
		exp = time.Now().Add(-1 * time.Minute)
	}
	claims := clayClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    testIssuer,
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	cc := GetClayContext(r.Context())
	if cc != nil {
		w.Header().Set("X-Got-User", cc.UserID)
		w.Header().Set("X-Got-Role", cc.Role)
	}
	w.WriteHeader(http.StatusOK)
}

func TestAuth_PublicRoute(t *testing.T) {
	rule := config.ParseAuthRule("none")
	h := Auth(testSecret, testIssuer, rule)(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuth_ValidToken(t *testing.T) {
	rule := config.ParseAuthRule("required")
	h := Auth(testSecret, testIssuer, rule)(http.HandlerFunc(okHandler))

	token := makeToken(t, "user-123", "user", false)
	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("X-Got-User") != "user-123" {
		t.Error("X-User-ID not injected into context")
	}
}

func TestAuth_MissingToken(t *testing.T) {
	rule := config.ParseAuthRule("required")
	h := Auth(testSecret, testIssuer, rule)(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	rule := config.ParseAuthRule("required")
	h := Auth(testSecret, testIssuer, rule)(http.HandlerFunc(okHandler))

	token := makeToken(t, "user-123", "user", true)
	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuth_WrongRole(t *testing.T) {
	rule := config.ParseAuthRule(`"required_roles:[driver]"`)
	h := Auth(testSecret, testIssuer, rule)(http.HandlerFunc(okHandler))

	token := makeToken(t, "user-123", "user", false)
	req := httptest.NewRequest(http.MethodPost, "/driver/orders/abc/accept", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAuth_CorrectRole(t *testing.T) {
	rule := config.ParseAuthRule(`"required_roles:[driver]"`)
	h := Auth(testSecret, testIssuer, rule)(http.HandlerFunc(okHandler))

	token := makeToken(t, "driver-456", "driver", false)
	req := httptest.NewRequest(http.MethodPost, "/driver/orders/abc/accept", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("X-Got-Role") != "driver" {
		t.Error("expected role=driver in context")
	}
}

func TestAuth_InvalidBearerFormat(t *testing.T) {
	rule := config.ParseAuthRule("required")
	h := Auth(testSecret, testIssuer, rule)(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	req.Header.Set("Authorization", "Token abc123") // wrong scheme
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all gateway configuration loaded from env vars.
type Config struct {
	Port         string
	JWTSecret    string
	JWTIssuer    string
	RedisAddr    string
	RedisPassword string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	// CORS allowed origins (comma-separated in env)
	CORSOrigins []string
}

// Load reads configuration from environment variables.
// All required vars must be set — returns error if any are missing.
func Load() (*Config, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	origins := os.Getenv("CORS_ORIGINS")
	if origins == "" {
		origins = "*"
	}

	return &Config{
		Port:          port,
		JWTSecret:     secret,
		JWTIssuer:     getEnvOrDefault("JWT_ISSUER", "clay-auth-service"),
		RedisAddr:     redisAddr,
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		ReadTimeout:   parseDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:  parseDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:   parseDuration("IDLE_TIMEOUT", 60*time.Second),
		CORSOrigins:   strings.Split(origins, ","),
	}, nil
}

// ─── Route config ─────────────────────────────────────────────────────────────

// Route represents one entry in routes.yaml.
type Route struct {
	Path        string   `yaml:"path"`
	Methods     []string `yaml:"methods"`
	Upstream    string   `yaml:"upstream"`
	Auth        string   `yaml:"auth"`
	RateLimit   int      `yaml:"rate_limit"`
	StripPrefix string   `yaml:"strip_prefix"`
}

// LoadRoutes parses routes.yaml from the given path.
func LoadRoutes(path string) ([]Route, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read routes file: %w", err)
	}

	var routes []Route
	if err := yaml.Unmarshal(data, &routes); err != nil {
		return nil, fmt.Errorf("parse routes file: %w", err)
	}

	if len(routes) == 0 {
		return nil, fmt.Errorf("routes file is empty")
	}

	return routes, nil
}

// ─── Auth rule parsing ────────────────────────────────────────────────────────

// AuthRule describes parsed auth requirements for a route.
type AuthRule struct {
	// Required: JWT must be valid.
	Required bool
	// Roles: if non-empty, JWT role must match one of these.
	Roles []string
}

// ParseAuthRule parses the auth field from routes.yaml.
//
// Supported formats:
//   - "none"                           → no auth
//   - "required"                       → any valid JWT
//   - `"required_roles:[user,driver]"` → JWT with matching role (quoted or unquoted)
func ParseAuthRule(auth string) AuthRule {
	// Strip surrounding quotes added by YAML for values containing colons
	auth = strings.Trim(auth, `"`)

	if auth == "none" || auth == "" {
		return AuthRule{}
	}
	if auth == "required" {
		return AuthRule{Required: true}
	}

	// required_roles:[role1,role2]
	if strings.HasPrefix(auth, "required_roles:[") && strings.HasSuffix(auth, "]") {
		inner := auth[len("required_roles:[") : len(auth)-1]
		roles := strings.Split(inner, ",")
		for i, r := range roles {
			roles[i] = strings.TrimSpace(r)
		}
		return AuthRule{Required: true, Roles: roles}
	}

	// fallback — treat as required
	return AuthRule{Required: true}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	// Accept plain seconds (e.g. "30") or Go duration (e.g. "30s")
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return def
}

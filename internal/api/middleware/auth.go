package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// APIKeyLookup resolves an API key hash to a workspace ID.
type APIKeyLookup interface {
	WorkspaceByKeyHash(ctx context.Context, hash string) (string, error)
}

// AuthConfig configures the auth middleware.
type AuthConfig struct {
	JWTSecret string
	Lookup    APIKeyLookup
}

type jwtClaims struct {
	jwt.RegisteredClaims
	WorkspaceID string `json:"workspace_id"`
}

// Auth validates Bearer tokens (JWT or aflow_ API keys) and sets workspace + user context.
// When AFLOW_AUTH_DISABLED=true it falls through to WorkspaceRequired (dev mode).
func Auth(cfg AuthConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if os.Getenv("AFLOW_AUTH_DISABLED") == "true" {
				return next(c)
			}

			raw := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(raw, "Bearer ") {
				return authErr(c, "Authorization: Bearer <token> required")
			}
			token := strings.TrimPrefix(raw, "Bearer ")

			var workspaceID, userID string

			if strings.HasPrefix(token, "aflow_") {
				// API key path.
				if cfg.Lookup == nil {
					return authErr(c, "API key auth not configured")
				}
				hash := hashKey(token)
				ws, err := cfg.Lookup.WorkspaceByKeyHash(c.Request().Context(), hash)
				if err != nil {
					return authErr(c, "invalid API key")
				}
				workspaceID = ws
			} else {
				// JWT path.
				if cfg.JWTSecret == "" {
					return authErr(c, "JWT auth not configured")
				}
				claims, err := parseJWT(token, cfg.JWTSecret)
				if err != nil {
					return authErr(c, "invalid token: "+err.Error())
				}
				workspaceID = claims.WorkspaceID
				userID = claims.Subject
			}

			if workspaceID == "" {
				return authErr(c, "token has no workspace_id claim")
			}

			// Overwrite any header-supplied values — auth is canonical.
			c.Set(string(WorkspaceIDKey), workspaceID)
			c.Set(string(UserIDKey), userID)
			return next(c)
		}
	}
}

// HashAPIKey returns the SHA-256 hex hash used to store/look up an API key.
func HashAPIKey(key string) string { return hashKey(key) }

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func parseJWT(tokenStr, secret string) (*jwtClaims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*jwtClaims)
	if !ok || !tok.Valid {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

func authErr(c echo.Context, detail string) error {
	return c.JSON(http.StatusUnauthorized, map[string]any{
		"type":   "https://aflow.dev/errors/unauthorized",
		"title":  "Unauthorized",
		"status": http.StatusUnauthorized,
		"detail": detail,
	})
}

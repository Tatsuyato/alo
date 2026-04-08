package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type contextKey string

const principalKey contextKey = "principal"

type apiKeyRecord struct {
	Principal Principal
}

type AuthService struct {
	apiKeys   map[string]apiKeyRecord
	jwtSecret string
	limiterMu sync.Mutex
	limiters  map[string]*rate.Limiter
}

func NewAuthService() *AuthService {
	adminKey := getEnv("API_KEY_ADMIN", "local-admin-key")
	deployerKey := getEnv("API_KEY_DEPLOYER", "local-deployer-key")

	return &AuthService{
		apiKeys: map[string]apiKeyRecord{
			adminKey: {
				Principal: Principal{
					Subject:     "admin",
					Role:        "admin",
					Namespaces:  []string{"*"},
					Permissions: []Permission{PermExecuteCommand, PermReadStatus, PermReadLogs, PermManageProject, PermBuildDeploy},
				},
			},
			deployerKey: {
				Principal: Principal{
					Subject:     "deployer-a",
					Role:        "deployer",
					Namespaces:  []string{"ns-a"},
					Permissions: []Permission{PermExecuteCommand, PermReadStatus, PermReadLogs, PermBuildDeploy},
				},
			},
		},
		jwtSecret: getEnv("JWT_SECRET", ""),
		limiters:  map[string]*rate.Limiter{},
	}
}

func (a *AuthService) authenticate(r *http.Request) (*Principal, error) {
	if key := strings.TrimSpace(r.Header.Get("X-API-Key")); key != "" {
		rec, ok := a.apiKeys[key]
		if !ok {
			return nil, errors.New("invalid api key")
		}
		p := rec.Principal
		return &p, nil
	}

	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer"))
		if token == "" {
			token = strings.TrimSpace(strings.TrimPrefix(authorization, "bearer"))
		}
		if token == "" {
			return nil, errors.New("missing bearer token")
		}
		return a.parseJWT(token)
	}

	return nil, errors.New("missing credentials")
}

func (a *AuthService) parseJWT(token string) (*Principal, error) {
	if a.jwtSecret == "" {
		return nil, errors.New("jwt disabled")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid jwt format")
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid jwt payload")
	}

	sigInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(a.jwtSecret))
	_, _ = mac.Write([]byte(sigInput))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, errors.New("invalid jwt signature")
	}

	var claims struct {
		Sub         string       `json:"sub"`
		Role        string       `json:"role"`
		Namespaces  []string     `json:"namespaces"`
		Permissions []Permission `json:"permissions"`
		Exp         int64        `json:"exp"`
	}
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return nil, errors.New("invalid jwt claims")
	}
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, errors.New("jwt expired")
	}
	if claims.Sub == "" {
		return nil, errors.New("jwt missing sub")
	}

	return &Principal{
		Subject:     claims.Sub,
		Role:        claims.Role,
		Namespaces:  claims.Namespaces,
		Permissions: claims.Permissions,
	}, nil
}

func (a *AuthService) limiterFor(subject string, rps float64, burst int) *rate.Limiter {
	a.limiterMu.Lock()
	defer a.limiterMu.Unlock()
	limiter, ok := a.limiters[subject]
	if !ok {
		limiter = rate.NewLimiter(rate.Limit(rps), burst)
		a.limiters[subject] = limiter
	}
	return limiter
}

func principalFromContext(ctx context.Context) (*Principal, bool) {
	v := ctx.Value(principalKey)
	p, ok := v.(*Principal)
	return p, ok
}

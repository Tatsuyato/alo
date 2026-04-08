package main

import (
	"context"
	"net/http"
)

func (a *App) withAuth(next http.Handler, permission Permission) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := a.auth.authenticate(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}

		if !hasPermission(principal.Permissions, permission) {
			writeError(w, http.StatusForbidden, "permission denied")
			return
		}

		ctx := context.WithValue(r.Context(), principalKey, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) withRateLimit(next http.Handler, rps float64, burst int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := principalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing auth context")
			return
		}
		limiter := a.auth.limiterFor(principal.Subject, rps, burst)
		if !limiter.Allow() {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

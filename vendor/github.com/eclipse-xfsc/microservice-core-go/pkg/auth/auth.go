package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Middleware is standard HTTP middleware used for authenticating
// requests carrying a bearer JWT token.
//
// It uses an internal caching mechanism for fetching Json Web Keys from
// a given URL and automatically refreshes the cache on a given time interval.
//
// JWT tokens are expected to carry a Header *kid* claim specifying the
// ID of the public key which should be used for verification.
type Middleware struct {
	jwkSet jwk.Set
}

func NewMiddleware(jwkURL string, refreshInterval time.Duration, c *http.Client) (*Middleware, error) {
	if jwkURL == "" {
		return nil, fmt.Errorf("missing JWK url")
	}

	cache := jwk.NewCache(context.Background())
	if err := cache.Register(jwkURL, jwk.WithHTTPClient(c), jwk.WithRefreshInterval(refreshInterval)); err != nil {
		return nil, fmt.Errorf("fail to register JWK url with cache: %v", err)
	}
	_, err := cache.Refresh(context.Background(), jwkURL)
	if err != nil {
		return nil, fmt.Errorf("fail to refresh JWK cache: %v", err)
	}

	return &Middleware{
		jwkSet: jwk.NewCachedSet(cache, jwkURL),
	}, nil
}

func (a *Middleware) Handler() func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := tokenFromRequest(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			_, err = jwt.Parse([]byte(token), jwt.WithKeySet(a.jwkSet))
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			h.ServeHTTP(w, r)
		})
	}
}

func tokenFromRequest(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	auth := strings.Split(authHeader, " ")
	if len(auth) != 2 {
		return "", fmt.Errorf("invalid authorization header")
	}

	if auth[0] != "Bearer" {
		return "", fmt.Errorf("invalid authorization header")
	}

	return auth[1], nil
}

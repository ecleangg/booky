package httpapi

import (
	"context"
	"crypto/rsa"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

type Principal struct {
	Subject     string
	WorkspaceID string
	Claims      jwt.MapClaims
}

type principalContextKey struct{}

type jwtVerifier struct {
	cfg      config.JWTConfig
	client   *http.Client
	logger   *slog.Logger
	mu       sync.RWMutex
	keys     map[string]*rsa.PublicKey
	fetchedAt time.Time
}

type jwksDocument struct {
	Keys []jsonWebKey `json:"keys"`
}

type jsonWebKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
	Use string `json:"use"`
}

func newJWTVerifier(cfg config.JWTConfig, logger *slog.Logger) *jwtVerifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &jwtVerifier{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

func withJWTAuth(verifier *jwtVerifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if verifier == nil || !verifier.cfg.Enabled {
			writeError(w, http.StatusServiceUnavailable, fmt.Errorf("jwt auth is not configured"))
			return
		}
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		scheme, token, ok := strings.Cut(authHeader, " ")
		if !ok || subtle.ConstantTimeCompare([]byte(strings.ToLower(strings.TrimSpace(scheme))), []byte("bearer")) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="booky-v1"`)
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing bearer token"))
			return
		}
		principal, err := verifier.Verify(r.Context(), strings.TrimSpace(token))
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="booky-v1"`)
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, principal)))
	})
}

func (v *jwtVerifier) Verify(ctx context.Context, tokenString string) (Principal, error) {
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method %s", token.Method.Alg())
		}
		return v.keyForToken(ctx, token)
	},
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}),
		jwt.WithIssuer(v.cfg.Issuer),
		jwt.WithAudience(v.cfg.Audience),
	)
	if err != nil {
		return Principal{}, fmt.Errorf("invalid token: %w", err)
	}
	if !parsed.Valid {
		return Principal{}, fmt.Errorf("invalid token")
	}
	subject, _ := claims[v.cfg.SubjectClaim].(string)
	workspaceID, _ := claims[v.cfg.WorkspaceClaim].(string)
	if strings.TrimSpace(subject) == "" {
		return Principal{}, fmt.Errorf("token missing %s claim", v.cfg.SubjectClaim)
	}
	if strings.TrimSpace(workspaceID) == "" {
		return Principal{}, fmt.Errorf("token missing %s claim", v.cfg.WorkspaceClaim)
	}
	return Principal{
		Subject:     subject,
		WorkspaceID: workspaceID,
		Claims:      claims,
	}, nil
}

func (v *jwtVerifier) keyForToken(ctx context.Context, token *jwt.Token) (*rsa.PublicKey, error) {
	kid, _ := token.Header["kid"].(string)
	if strings.TrimSpace(kid) == "" {
		return nil, fmt.Errorf("token missing kid")
	}
	keys, err := v.keysFor(ctx, false)
	if err != nil {
		return nil, err
	}
	if key, ok := keys[kid]; ok {
		return key, nil
	}
	keys, err = v.keysFor(ctx, true)
	if err != nil {
		return nil, err
	}
	key, ok := keys[kid]
	if !ok {
		return nil, fmt.Errorf("kid %q not found in jwks", kid)
	}
	return key, nil
}

func (v *jwtVerifier) keysFor(ctx context.Context, force bool) (map[string]*rsa.PublicKey, error) {
	v.mu.RLock()
	if !force && len(v.keys) > 0 && time.Since(v.fetchedAt) < 15*time.Minute {
		defer v.mu.RUnlock()
		return cloneKeys(v.keys), nil
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()
	if !force && len(v.keys) > 0 && time.Since(v.fetchedAt) < 15*time.Minute {
		return cloneKeys(v.keys), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.JWKSURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create jwks request: %w", err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch jwks returned %d", resp.StatusCode)
	}
	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, jwk := range doc.Keys {
		if jwk.Kty != "RSA" || jwk.Kid == "" {
			continue
		}
		key, err := rsaKeyFromJWK(jwk)
		if err != nil {
			if v.logger != nil {
				v.logger.Warn("skip invalid jwk", "kid", jwk.Kid, "error", err)
			}
			continue
		}
		keys[jwk.Kid] = key
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("jwks did not contain usable rsa keys")
	}
	v.keys = keys
	v.fetchedAt = time.Now()
	return cloneKeys(v.keys), nil
}

func cloneKeys(in map[string]*rsa.PublicKey) map[string]*rsa.PublicKey {
	out := make(map[string]*rsa.PublicKey, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func rsaKeyFromJWK(jwk jsonWebKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}
	e := 0
	for _, value := range eBytes {
		e = e<<8 + int(value)
	}
	if e == 0 {
		return nil, fmt.Errorf("rsa exponent is zero")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}, nil
}

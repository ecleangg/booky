package httpapi

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTAuthAcceptsValidTokenAndInjectsPrincipal(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{
					"kid": "test-key",
					"kty": "RSA",
					"use": "sig",
					"n":   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
				},
			},
		})
	}))
	defer jwksServer.Close()

	verifier := newJWTVerifier(config.JWTConfig{
		Enabled:        true,
		Issuer:         "https://issuer.example",
		Audience:       "booky-api",
		JWKSURL:        jwksServer.URL,
		SubjectClaim:   "sub",
		WorkspaceClaim: "workspace_id",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":          "https://issuer.example",
		"aud":          "booky-api",
		"sub":          "user_123",
		"workspace_id": "ws_123",
		"iat":          time.Now().Add(-1 * time.Minute).Unix(),
		"nbf":          time.Now().Add(-1 * time.Minute).Unix(),
		"exp":          time.Now().Add(1 * time.Hour).Unix(),
	})
	token.Header["kid"] = "test-key"
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("SignedString returned error: %v", err)
	}

	handler := withJWTAuth(verifier, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			t.Fatal("expected principal in request context")
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"subject":      principal.Subject,
			"workspace_id": principal.WorkspaceID,
		})
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/stripe/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var payload map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if payload["subject"] != "user_123" {
		t.Fatalf("unexpected subject %q", payload["subject"])
	}
	if payload["workspace_id"] != "ws_123" {
		t.Fatalf("unexpected workspace_id %q", payload["workspace_id"])
	}
}

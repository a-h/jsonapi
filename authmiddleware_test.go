package jsonapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthMiddleware(t *testing.T) {
	t.Run("errors if the token fetcher fails", func(t *testing.T) {
		var called bool
		m := newAuthMiddleware(func() (string, error) {
			called = true
			return "", nil
		})
		err := m.Request(nil)
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
		if !called {
			t.Errorf("expected the token fetcher to be called, but it wasn't")
		}
	})
	t.Run("errors if the token is not a JWT", func(t *testing.T) {
		m := newAuthMiddleware(func() (string, error) {
			return "not-a-jwt", nil
		})
		err := m.Request(nil)
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	})
	t.Run("errors if the token JWT section is not valid base64", func(t *testing.T) {
		m := newAuthMiddleware(func() (string, error) {
			return "header.<>.signature", nil
		})
		err := m.Request(nil)
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	})
	t.Run("errors if the token JWT section is not valid JSON", func(t *testing.T) {
		m := newAuthMiddleware(func() (string, error) {
			return "header." + base64.RawURLEncoding.EncodeToString([]byte("not-json")) + ".signature", nil
		})
		err := m.Request(nil)
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	})

	now := func() time.Time {
		return time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	t.Run("calls the token fetcher if it has not been called before", func(t *testing.T) {
		var called bool
		m := newAuthMiddleware(func() (string, error) {
			called = true
			return "", nil
		})
		m.now = now
		_ = m.Request(nil)
		if !called {
			t.Errorf("expected the token fetcher to be called, but it wasn't")
		}
	})

	validClaimsJSON, err := json.Marshal(map[string]any{
		"exp": now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("failed to marshal valid claims: %v", err)
	}
	validToken := "header." + base64.RawURLEncoding.EncodeToString(validClaimsJSON) + ".signature"

	expiredClaimsJSON, err := json.Marshal(map[string]any{
		"exp": now().Add(-time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("failed to marshal expired claims: %v", err)
	}
	expiredToken := "header." + base64.RawURLEncoding.EncodeToString(expiredClaimsJSON) + ".signature"

	t.Run("calls the token fetcher if the token has expired", func(t *testing.T) {
		var responses = []string{expiredToken, validToken}
		var callCount int
		m := newAuthMiddleware(func() (string, error) {
			defer func() {
				callCount++
			}()
			return responses[callCount], nil
		})
		m.now = now

		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

		// Call once to setup with an expired token.
		if err = m.Request(req); err != nil {
			t.Fatalf("setup expected no error, got %v", err)
		}

		// Check that the token fetcher is called again.
		if err := m.Request(req); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if callCount != 2 {
			t.Errorf("expected the token fetcher to be called twice, but it was called %d times", callCount)
		}
		if req.Header.Get("Authorization") != "Bearer "+validToken {
			t.Errorf("expected the request to have the new token, got %v", req.Header.Get("Authorization"))
		}
	})
	t.Run("does not call the token fetcher if the token has not expired", func(t *testing.T) {
		var callCount int
		m := newAuthMiddleware(func() (string, error) {
			defer func() {
				callCount++
			}()
			return validToken, nil
		})
		m.now = now

		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

		// Call once to setup with a valid token.
		if err = m.Request(req); err != nil {
			t.Fatalf("setup expected no error, got %v", err)
		}

		// Check that the token fetcher is not called again.
		for i := 0; i < 3; i++ {
			if err := m.Request(req); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		}
		if callCount != 1 {
			t.Errorf("expected the token fetcher to be called once, but it was called %d times", callCount)
		}
	})
	t.Run("proides a token to the request", func(t *testing.T) {
		m := newAuthMiddleware(func() (string, error) {
			return validToken, nil
		})
		m.now = now

		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

		if err = m.Request(req); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if req.Header.Get("Authorization") != "Bearer "+validToken {
			t.Errorf("expected the request to have the token, got %v", req.Header.Get("Authorization"))
		}
	})
}

func TestWithAuthMiddleware(t *testing.T) {
	tokenFetcher := func() (string, error) {
		return "tf123", nil
	}
	mw := WithAuthMiddleware(tokenFetcher)
	config := &Config{}
	err := mw(config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	var authMiddlewareFound bool
	for _, mw := range config.Middleware {
		if amw, ok := mw.(*AuthMiddleware); ok {
			tok, err := amw.TokenFetcher()
			if err != nil {
				t.Fatalf("expected no token fetch error, got %q", err)
			}
			if tok != "tf123" {
				t.Errorf("expected the token to be 'tf123', got %q", tok)
			}
			authMiddlewareFound = true
		}
	}
	if !authMiddlewareFound {
		t.Error("expected the auth middleware to be added to the config, but it wasn't")
	}
}

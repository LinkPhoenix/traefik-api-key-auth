package traefik_api_key_auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateConfigDefaults(t *testing.T) {
	cfg := CreateConfig()
	if cfg.QueryParam {
		t.Fatal("queryParam should default to false")
	}
	if cfg.PathSegment {
		t.Fatal("pathSegment should default to false")
	}
	if !cfg.AuthenticationHeader || !cfg.BearerHeader {
		t.Fatal("header and bearer should default to true")
	}
}

func TestFindMatchingKey(t *testing.T) {
	index := buildKeyIndex([]string{"key1", "secret-key", "abc"})
	tests := []struct {
		name     string
		provided string
		want     string
	}{
		{"exact match first", "key1", "key1"},
		{"exact match middle", "secret-key", "secret-key"},
		{"no match", "wrong", ""},
		{"empty provided", "", ""},
		{"prefix only no match", "key", ""},
		{"substring no match", "secret", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMatchingKey(tt.provided, index)
			if got != tt.want {
				t.Errorf("findMatchingKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"Bearer mytoken", "mytoken"},
		{"Bearer   spaced ", "spaced"},
		{"Bearer x", "x"},
		{"bearer lower", "lower"},
		{"BEARER upper", "upper"},
		{"Bearer too many spaces token", ""},
		{"Invalid", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractBearerToken(tt.header)
		if got != tt.want {
			t.Errorf("extractBearerToken(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestPathSegmentMatchesKey(t *testing.T) {
	index := buildKeyIndex([]string{"api-key-123", "token"})
	tests := []struct {
		path string
		want string
	}{
		{"/api/api-key-123/foo", "api-key-123"},
		{"/api-key-123", "api-key-123"},
		{"/prefix/api-key-123", "api-key-123"},
		{"/no/match/here", ""},
		{"/api/token", "token"},
		{"/token/rest", "token"},
	}
	for _, tt := range tests {
		got := pathSegmentMatchesKey(tt.path, index)
		if got != tt.want {
			t.Errorf("pathSegmentMatchesKey(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestResolveKeys(t *testing.T) {
	got, err := resolveKeys([]string{"a", "b", "a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("resolveKeys static = %v", got)
	}

	t.Setenv("TEST_API_KEY_PLUGIN", "from-env")
	got, err = resolveKeys([]string{"static", "env:TEST_API_KEY_PLUGIN"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "static" || got[1] != "from-env" {
		t.Errorf("resolveKeys with env = %v", got)
	}

	_, err = resolveKeys([]string{"env:NOT_SET_VAR"})
	if err == nil {
		t.Error("resolveKeys with only unset env should error")
	}
}

func TestNew(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	ctx := context.Background()

	_, err := New(ctx, next, nil, "test")
	if err == nil {
		t.Error("New with nil config should error")
	}

	_, err = New(ctx, next, &Config{Keys: nil}, "test")
	if err == nil {
		t.Error("New with no keys should error")
	}

	_, err = New(ctx, next, &Config{Keys: []string{}}, "test")
	if err == nil {
		t.Error("New with empty keys should error")
	}

	_, err = New(ctx, next, &Config{
		Keys:                 []string{"k"},
		AuthenticationHeader: false,
		BearerHeader:         false,
		QueryParam:           false,
		PathSegment:          false,
	}, "test")
	if err == nil {
		t.Error("New with no method enabled should error")
	}

	_, err = New(ctx, next, &Config{Keys: []string{"k"}, AuthenticationHeader: true, AuthenticationHeaderName: ""}, "test")
	if err == nil {
		t.Error("New should error when authentication header name is empty")
	}

	_, err = New(ctx, next, &Config{Keys: []string{"k"}, BearerHeader: true, BearerHeaderName: ""}, "test")
	if err == nil {
		t.Error("New should error when bearer header name is empty")
	}

	_, err = New(ctx, next, &Config{Keys: []string{"k"}, QueryParam: true, QueryParamName: ""}, "test")
	if err == nil {
		t.Error("New should error when query param name is empty")
	}

	_, err = New(ctx, next, &Config{Keys: []string{"k"}, AuthenticationHeader: true, AuthenticationHeaderName: "X-API-KEY"}, "test")
	if err != nil {
		t.Errorf("New valid config: %v", err)
	}
}

func TestKeyAuthServeHTTP(t *testing.T) {
	ctx := context.Background()

	t.Run("valid header removes source header and overwrites internal forward header", func(t *testing.T) {
		var receivedInternal string
		var receivedAuth string
		var internalHeaderValues int
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedInternal = r.Header.Get("X-Internal-Key")
			receivedAuth = r.Header.Get("X-API-KEY")
			internalHeaderValues = len(r.Header.Values("X-Internal-Key"))
			w.WriteHeader(http.StatusOK)
		})

		h, err := New(ctx, next, &Config{
			Keys:                      []string{"valid-key"},
			AuthenticationHeader:      true,
			AuthenticationHeaderName:  "X-API-KEY",
			BearerHeader:              false,
			QueryParam:                false,
			PathSegment:               false,
			RemoveHeadersOnSuccess:    true,
			InternalForwardHeaderName: "X-Internal-Key",
		}, "test")
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Add("X-Internal-Key", "attacker-value")
		req.Header.Set("X-API-KEY", "valid-key")
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, req)

		if rw.Code != http.StatusOK {
			t.Fatalf("status = %d", rw.Code)
		}
		if receivedAuth != "" {
			t.Fatalf("expected auth header to be removed, got %q", receivedAuth)
		}
		if receivedInternal != "valid-key" {
			t.Fatalf("expected internal header %q, got %q", "valid-key", receivedInternal)
		}
		if internalHeaderValues != 1 {
			t.Fatalf("expected exactly one internal header value, got %d", internalHeaderValues)
		}
	})

	t.Run("valid bearer token allows request", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		h, err := New(ctx, next, &Config{
			Keys:                 []string{"valid-key"},
			AuthenticationHeader: false,
			BearerHeader:         true,
			BearerHeaderName:     "Authorization",
			QueryParam:           false,
			PathSegment:          false,
		}, "test")
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "bearer valid-key")
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, req)

		if !nextCalled {
			t.Fatal("next handler should be called")
		}
	})

	t.Run("valid query param strips token", func(t *testing.T) {
		var rawQuery string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusOK)
		})

		h, err := New(ctx, next, &Config{
			Keys:                 []string{"valid-key"},
			AuthenticationHeader: false,
			BearerHeader:         false,
			QueryParam:           true,
			QueryParamName:       "token",
			PathSegment:          false,
		}, "test")
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/?token=valid-key&x=1", nil)
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, req)

		if rw.Code != http.StatusOK {
			t.Fatalf("status = %d", rw.Code)
		}
		if rawQuery != "x=1" {
			t.Fatalf("expected query to be sanitized, got %q", rawQuery)
		}
	})

	t.Run("valid path segment allows request", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		h, err := New(ctx, next, &Config{
			Keys:                 []string{"valid-key"},
			AuthenticationHeader: false,
			BearerHeader:         false,
			QueryParam:           false,
			PathSegment:          true,
		}, "test")
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/valid-key/v1", nil)
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, req)

		if !nextCalled {
			t.Fatal("next handler should be called")
		}
	})

	t.Run("invalid key returns 403", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		h, err := New(ctx, next, &Config{
			Keys:                     []string{"valid-key"},
			AuthenticationHeader:     true,
			AuthenticationHeaderName: "X-API-KEY",
			BearerHeader:             false,
			QueryParam:               false,
			PathSegment:              false,
		}, "test")
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-KEY", "wrong")
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, req)

		if nextCalled {
			t.Fatal("next handler should not be called")
		}
		if rw.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rw.Code)
		}
	})

	t.Run("permissive mode allows invalid request", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		h, err := New(ctx, next, &Config{
			Keys:                     []string{"valid-key"},
			AuthenticationHeader:     true,
			AuthenticationHeaderName: "X-API-KEY",
			BearerHeader:             false,
			QueryParam:               false,
			PathSegment:              false,
			PermissiveMode:           true,
		}, "test")
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-KEY", "wrong")
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, req)

		if !nextCalled {
			t.Fatal("next handler should be called")
		}
		if rw.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rw.Code)
		}
	})

	t.Run("internal error route rewrites path and clears query", func(t *testing.T) {
		var nextPath string
		var nextQuery string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextPath = r.URL.Path
			nextQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusOK)
		})

		h, err := New(ctx, next, &Config{
			Keys:                     []string{"valid-key"},
			AuthenticationHeader:     true,
			AuthenticationHeaderName: "X-API-KEY",
			BearerHeader:             false,
			QueryParam:               false,
			PathSegment:              false,
			InternalErrorRoute:       "errors/forbidden",
		}, "test")
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/private?token=wrong", nil)
		req.Header.Set("X-API-KEY", "wrong")
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, req)

		if nextPath != "/errors/forbidden" {
			t.Fatalf("path = %q, want /errors/forbidden", nextPath)
		}
		if nextQuery != "" {
			t.Fatalf("query should be cleared, got %q", nextQuery)
		}
	})

	t.Run("exempt paths are normalized and skip auth", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		h, err := New(ctx, next, &Config{
			Keys:                     []string{"valid-key"},
			AuthenticationHeader:     true,
			AuthenticationHeaderName: "X-API-KEY",
			BearerHeader:             false,
			QueryParam:               false,
			PathSegment:              false,
			ExemptPaths:              []string{" health/ ", "/metrics/"},
		}, "test")
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, req)
		if !nextCalled {
			t.Fatal("next handler should be called for /health")
		}

		nextCalled = false
		req = httptest.NewRequest(http.MethodGet, "/metrics/node", nil)
		rw = httptest.NewRecorder()
		h.ServeHTTP(rw, req)
		if !nextCalled {
			t.Fatal("next handler should be called for /metrics/node")
		}
	})
}

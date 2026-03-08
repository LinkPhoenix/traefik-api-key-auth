package traefik_api_key_auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func Test_constantTimeContains(t *testing.T) {
	keys := []string{"key1", "secret-key", "abc"}
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
			got := constantTimeContains(tt.provided, keys)
			if got != tt.want {
				t.Errorf("constantTimeContains() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_extractBearerToken(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"Bearer mytoken", "mytoken"},
		{"Bearer   spaced ", "spaced"},
		{"Bearer x", "x"},
		{"bearer lower", ""},
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

func Test_pathSegmentMatchesKey(t *testing.T) {
	keys := []string{"api-key-123", "token"}
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
		got := pathSegmentMatchesKey(tt.path, keys)
		if got != tt.want {
			t.Errorf("pathSegmentMatchesKey(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func Test_resolveKeys(t *testing.T) {
	// Static keys only
	got, err := resolveKeys([]string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("resolveKeys static = %v", got)
	}

	// With env
	os.Setenv("TEST_API_KEY_PLUGIN", "from-env")
	defer os.Unsetenv("TEST_API_KEY_PLUGIN")
	got, err = resolveKeys([]string{"static", "env:TEST_API_KEY_PLUGIN"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "static" || got[1] != "from-env" {
		t.Errorf("resolveKeys with env = %v", got)
	}

	// Empty after resolve
	_, err = resolveKeys([]string{"env:NOT_SET_VAR"})
	if err == nil {
		t.Error("resolveKeys with only unset env should error")
	}
}

func Test_New(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	ctx := context.Background()

	_, err := New(ctx, next, &Config{Keys: nil}, "test")
	if err == nil {
		t.Error("New with no keys should error")
	}

	_, err = New(ctx, next, &Config{Keys: []string{}}, "test")
	if err == nil {
		t.Error("New with empty keys should error")
	}

	_, err = New(ctx, next, &Config{
		Keys:                []string{"k"},
		AuthenticationHeader: false,
		BearerHeader:        false,
		QueryParam:          false,
		PathSegment:         false,
	}, "test")
	if err == nil {
		t.Error("New with no method enabled should error")
	}

	_, err = New(ctx, next, &Config{Keys: []string{"k"}, AuthenticationHeader: true}, "test")
	if err != nil {
		t.Errorf("New valid config: %v", err)
	}
}

func Test_KeyAuth_ServeHTTP(t *testing.T) {
	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	ctx := context.Background()
	config := &Config{
		Keys:                 []string{"valid-key"},
		AuthenticationHeader: true,
		AuthenticationHeaderName: "X-API-KEY",
		BearerHeader:         true,
		QueryParam:           true,
		QueryParamName:       "token",
		PathSegment:          false,
	}
	ka, err := New(ctx, next, config, "test")
	if err != nil {
		t.Fatal(err)
	}
	handler := ka.(*KeyAuth)

	t.Run("valid header", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-KEY", "valid-key")
		rw := httptest.NewRecorder()
		handler.ServeHTTP(rw, req)
		if !nextCalled {
			t.Error("next handler should be called")
		}
		if rw.Code != http.StatusOK {
			t.Errorf("status = %d", rw.Code)
		}
	})

	t.Run("invalid key returns 403", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-KEY", "wrong")
		rw := httptest.NewRecorder()
		handler.ServeHTTP(rw, req)
		if nextCalled {
			t.Error("next handler should not be called")
		}
		if rw.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rw.Code)
		}
	})

	t.Run("exempt path skips auth", func(t *testing.T) {
		configExempt := &Config{
			Keys:                 []string{"valid-key"},
			AuthenticationHeader: true,
			AuthenticationHeaderName: "X-API-KEY",
			ExemptPaths:          []string{"/health"},
		}
		ka2, _ := New(ctx, next, configExempt, "test")
		handler2 := ka2.(*KeyAuth)
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rw := httptest.NewRecorder()
		handler2.ServeHTTP(rw, req)
		if !nextCalled {
			t.Error("next handler should be called for exempt path")
		}
	})
}

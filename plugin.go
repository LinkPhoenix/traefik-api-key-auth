package traefik_api_key_auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// bearerRegex extracts the token from "Authorization: Bearer <token>". Compiled once to avoid per-request allocation.
var bearerRegex = regexp.MustCompile(`Bearer\s+(?P<key>\S+)`)

type Config struct {
	AuthenticationHeader      bool     `json:"authenticationHeader,omitempty"`
	AuthenticationHeaderName  string   `json:"authenticationHeaderName,omitempty"`
	BearerHeader              bool     `json:"bearerHeader,omitempty"`
	BearerHeaderName          string   `json:"bearerHeaderName,omitempty"`
	QueryParam                bool     `json:"queryParam,omitempty"`
	QueryParamName            string   `json:"queryParamName,omitempty"`
	PathSegment               bool     `json:"pathSegment,omitempty"`
	PermissiveMode            bool     `json:"permissiveMode,omitempty"`
	Keys                      []string `json:"keys,omitempty"`
	RemoveHeadersOnSuccess    bool     `json:"removeHeadersOnSuccess,omitempty"`
	InternalForwardHeaderName string   `json:"internalForwardHeaderName,omitempty"`
	InternalErrorRoute        string   `json:"internalErrorRoute,omitempty"`
	ExemptPaths               []string `json:"exemptPaths,omitempty"`
}

type Response struct {
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
}

func CreateConfig() *Config {
	return &Config{
		AuthenticationHeader:      true,
		AuthenticationHeaderName:  "X-API-KEY",
		BearerHeader:              true,
		BearerHeaderName:          "Authorization",
		QueryParam:                true,
		QueryParamName:            "token",
		PathSegment:               true,
		PermissiveMode:            false,
		Keys:                      make([]string, 0),
		RemoveHeadersOnSuccess:    true,
		InternalForwardHeaderName: "",
		InternalErrorRoute:        "",
		ExemptPaths:               nil,
	}
}

type KeyAuth struct {
	next                      http.Handler
	authenticationHeader      bool
	authenticationHeaderName  string
	bearerHeader              bool
	bearerHeaderName          string
	queryParam                bool
	queryParamName            string
	pathSegment               bool
	permissiveMode            bool
	keys                      []string
	removeHeadersOnSuccess    bool
	internalForwardHeaderName string
	internalErrorRoute        string
	exemptPaths               []string
}

// resolveKeys expands config keys: entries "env:VAR_NAME" are replaced by the value of the environment variable.
// Returns the final list of keys and an error if no keys remain.
func resolveKeys(rawKeys []string) ([]string, error) {
	var keys []string
	for _, k := range rawKeys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if strings.HasPrefix(k, "env:") {
			envVar := strings.TrimSpace(strings.TrimPrefix(k, "env:"))
			if envVar != "" {
				if v := os.Getenv(envVar); v != "" {
					keys = append(keys, v)
				}
			}
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("must specify at least one valid key (or use env:VAR_NAME)")
	}
	return keys, nil
}

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	// Do not log config to avoid leaking keys
	_, _ = os.Stdout.WriteString("traefik_api_key_auth: creating plugin " + name + "\n")

	resolvedKeys, err := resolveKeys(config.Keys)
	if err != nil {
		return nil, err
	}

	if !config.AuthenticationHeader && !config.BearerHeader && !config.QueryParam && !config.PathSegment {
		return nil, fmt.Errorf("at least one method must be true")
	}

	return &KeyAuth{
		next:                      next,
		authenticationHeader:      config.AuthenticationHeader,
		authenticationHeaderName:  config.AuthenticationHeaderName,
		bearerHeader:              config.BearerHeader,
		bearerHeaderName:          config.BearerHeaderName,
		queryParam:                config.QueryParam,
		queryParamName:            config.QueryParamName,
		pathSegment:               config.PathSegment,
		permissiveMode:            config.PermissiveMode,
		keys:                      resolvedKeys,
		removeHeadersOnSuccess:    config.RemoveHeadersOnSuccess,
		internalForwardHeaderName: config.InternalForwardHeaderName,
		internalErrorRoute:        config.InternalErrorRoute,
		exemptPaths:               config.ExemptPaths,
	}, nil
}

// constantTimeContains returns the matching valid key if the provided key matches any of them using constant-time comparison.
// Empty provided key never matches. Used to prevent timing attacks.
func constantTimeContains(provided string, validKeys []string) string {
	if provided == "" {
		return ""
	}
	providedB := []byte(provided)
	for _, valid := range validKeys {
		if len(providedB) != len(valid) {
			continue
		}
		if subtle.ConstantTimeCompare(providedB, []byte(valid)) == 1 {
			return valid
		}
	}
	return ""
}

// extractBearerToken returns the token from "Authorization: Bearer <token>" or empty string if not in that form.
func extractBearerToken(headerValue string) string {
	matches := bearerRegex.FindStringSubmatch(strings.TrimSpace(headerValue))
	if matches == nil {
		return ""
	}
	idx := bearerRegex.SubexpIndex("key")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(matches[idx])
}

// pathSegmentMatchesKey returns the matching key if any path segment (between slashes) exactly matches a valid key.
// Uses constant-time comparison; avoids substring matching for security.
func pathSegmentMatchesKey(path string, validKeys []string) string {
	segments := strings.Split(path, "/")
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if matched := constantTimeContains(seg, validKeys); matched != "" {
			return matched
		}
	}
	return ""
}

func (ka *KeyAuth) ok(rw http.ResponseWriter, req *http.Request, matchedKey string) {
	// Do not log the key to avoid leaking secrets
	_, _ = os.Stdout.WriteString("traefik_api_key_auth: valid credentials for URL " + req.URL.String() + "\n")
	if ka.internalForwardHeaderName != "" {
		req.Header.Add(ka.internalForwardHeaderName, matchedKey)
	}
	req.RequestURI = req.URL.RequestURI()
	ka.next.ServeHTTP(rw, req)
}

func (ka *KeyAuth) permissiveOk(rw http.ResponseWriter, req *http.Request) {
	_, _ = os.Stderr.WriteString("traefik_api_key_auth: no valid credentials for URL \"" + req.URL.String() + "\"; allowing in permissive mode\n")
	req.RequestURI = req.URL.RequestURI()
	ka.next.ServeHTTP(rw, req)
}

func (ka *KeyAuth) isExempt(path string) bool {
	for _, prefix := range ka.exemptPaths {
		if prefix == "" {
			continue
		}
		if path == prefix || strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")+"/") || strings.HasPrefix(path, prefix+"/") {
			return true
		}
		if path == strings.TrimSuffix(prefix, "/") {
			return true
		}
	}
	return false
}

func (ka *KeyAuth) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if ka.isExempt(path) {
		req.RequestURI = req.URL.RequestURI()
		ka.next.ServeHTTP(rw, req)
		return
	}

	if ka.authenticationHeader {
		provided := req.Header.Get(ka.authenticationHeaderName)
		if matched := constantTimeContains(provided, ka.keys); matched != "" {
			if ka.removeHeadersOnSuccess {
				req.Header.Del(ka.authenticationHeaderName)
			}
			ka.ok(rw, req, matched)
			return
		}
	}

	if ka.bearerHeader {
		token := extractBearerToken(req.Header.Get(ka.bearerHeaderName))
		if matched := constantTimeContains(token, ka.keys); matched != "" {
			if ka.removeHeadersOnSuccess {
				req.Header.Del(ka.bearerHeaderName)
			}
			ka.ok(rw, req, matched)
			return
		}
	}

	if ka.queryParam {
		qs := req.URL.Query()
		provided := qs.Get(ka.queryParamName)
		if matched := constantTimeContains(provided, ka.keys); matched != "" {
			qs.Del(ka.queryParamName)
			req.URL.RawQuery = qs.Encode()
			ka.ok(rw, req, matched)
			return
		}
	}

	if ka.pathSegment {
		if matched := pathSegmentMatchesKey(path, ka.keys); matched != "" {
			ka.ok(rw, req, matched)
			return
		}
	}

	if ka.permissiveMode {
		ka.permissiveOk(rw, req)
		return
	}

	if ka.internalErrorRoute != "" {
		req.URL.Path = ka.internalErrorRoute
		req.URL.RawQuery = ""
		req.RequestURI = req.URL.RequestURI()
		ka.next.ServeHTTP(rw, req)
		return
	}

	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.WriteHeader(http.StatusForbidden)
	response := Response{Message: "Invalid or missing API Key", StatusCode: http.StatusForbidden}
	if err := json.NewEncoder(rw).Encode(response); err != nil {
		_, _ = os.Stderr.WriteString("traefik_api_key_auth: failed to write response: " + err.Error() + "\n")
	}
}

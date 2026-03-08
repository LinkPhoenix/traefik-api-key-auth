# traefik-api-key-auth

Traefik plugin that protects routes with an API key provided via:
- an authentication header
- a bearer token
- a query parameter (optional)
- a path segment (optional)

Keys are validated with constant-time comparison. If no valid key is provided, the middleware returns `403` (or forwards to an internal error route if configured).

## Security Notes

- Do not commit API keys in configuration files.
- Prefer `env:VAR_NAME` in `keys` to load secrets from environment variables.
- Query parameter and path segment methods are disabled by default to avoid key leakage in logs, caches, and history.
- The plugin removes matched credentials from request headers/query before forwarding when configured.

## Config

### Static file

Add to your Traefik static configuration.

#### yaml

```yaml
experimental:
  plugins:
    traefik-api-key-auth:
      moduleName: "github.com/linkphoenix/traefik-api-key-auth"
      version: "v0.1.0"
```

#### toml

```toml
[experimental.plugins.traefik-api-key-auth]
  moduleName = "github.com/linkphoenix/traefik-api-key-auth"
  version = "v0.1.0"
```

### CLI

```sh
--experimental.plugins.traefik-api-key-auth.modulename=github.com/linkphoenix/traefik-api-key-auth
--experimental.plugins.traefik-api-key-auth.version=v0.1.0
```

### K8s CRD (production-safe example)

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: verify-api-key
spec:
  plugin:
    traefik-api-key-auth:
      authenticationHeader: true
      authenticationHeaderName: X-API-KEY
      bearerHeader: true
      bearerHeaderName: Authorization
      queryParam: false
      queryParamName: token
      pathSegment: false
      permissiveMode: false
      removeHeadersOnSuccess: true
      internalForwardHeaderName: X-Validated-Api-Key
      internalErrorRoute: ''
      exemptPaths:
        - /health
      keys:
        - env:MY_API_KEY
```

## Validation Order

The middleware checks credentials in this order:
1. `authenticationHeader`
2. `bearerHeader`
3. `queryParam`
4. `pathSegment`

The first valid key wins.

## Plugin options

| option                      | default           | type     | description                                                                  | optional |
|:----------------------------|:------------------|:---------|:-----------------------------------------------------------------------------|:---------|
| `authenticationHeader`      | `true`            | bool     | Use an authentication header to pass a valid key.                            | ⚠️       |
| `authenticationHeaderName`  | `"X-API-KEY"`     | string   | The name of the authentication header.                                       | ✅       |
| `bearerHeader`              | `true`            | bool     | Use an authorization header to pass a bearer token (key).                    | ⚠️       |
| `bearerHeaderName`          | `"Authorization"` | string   | The name of the authorization bearer header.                                 | ✅       |
| `queryParam`                | `false`           | bool     | Use a query string param to pass a valid key.                                | ⚠️       |
| `queryParamName`            | `"token"`         | string   | The name of the query string param.                                          | ✅       |
| `pathSegment`               | `false`           | bool     | Use exact path segment match to pass a valid key.                            | ⚠️       |
| `permissiveMode`            | `false`           | bool     | Allow requests even when credentials are invalid/missing (dry-run mode).     | ✅       |
| `removeHeadersOnSuccess`    | `true`            | bool     | Remove source credential headers on successful auth.                         | ✅       |
| `internalForwardHeaderName` | `""`              | string   | Optionally forward validated key as an internal header to next middleware.   | ✅       |
| `internalErrorRoute`        | `""`              | string   | On invalid key, forward request to this internal path instead of returning 403. | ✅    |
| `exemptPaths`               | `[]`              | []string | Path prefixes that skip authentication (example: `/health`, `/metrics`).     | ✅       |
| `keys`                      | `[]`              | []string | Valid keys. Use `env:VAR_NAME` to load a key from environment variable.      | ❌       |

`⚠️` At least one of `authenticationHeader`, `bearerHeader`, `queryParam`, or `pathSegment` must be `true`.

`❌` Required.

`✅` Optional.

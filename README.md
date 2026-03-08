# traefik-api-key-auth

Traefik plugin that protects routes with an API key provided via a header, query parameter, or path segment. Keys are validated using constant-time comparison to reduce timing attacks. If no valid key is provided, the middleware returns 403 (or forwards to an internal error route if configured).

**Security:** Do not commit keys in config files. Prefer `env:VAR_NAME` in the `keys` list to load keys from environment variables in production.

## Config

### Static file

Add to your Traefik static configuration

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

Add to your startup args:

```sh
--experimental.plugins.traefik-api-key-auth.modulename=github.com/linkphoenix/traefik-api-key-auth
--experimental.plugins.traefik-api-key-auth.version=v0.1.0
```

### K8s CRD

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
      queryParam: true
      queryParamName: token
      pathSegment: true
      permissiveMode: false
      removeHeadersOnSuccess: true
      internalForwardHeaderName: ''
      internalErrorRoute: ''
      exemptPaths: []
      keys:
        - some-api-key
        # Or load from environment: - env:MY_API_KEY
```

## Plugin options

| option                      | default           | type     | description                                                       | optional |
|:----------------------------|:------------------|:---------|:------------------------------------------------------------------|:---------|
| `authenticationHeader`      | `true`            | bool     | Use an authentication header to pass a valid key.                 | ⚠️        |
| `authenticationHeaderName`  | `"X-API-KEY"`     | string   | The name of the authentication header.                            | ✅       |
| `bearerHeader`              | `true`            | bool     | Use an authorization header to pass a bearer token (key).         | ⚠️        |
| `bearerHeaderName`          | `"Authorization"` | string   | The name of the authorization bearer header.                      | ✅       |
| `queryParam`                | `true`            | bool     | Use a query string param to pass a valid key.                     | ⚠️        |
| `queryParamName`            | `"token"`         | string   | The name of the query string param.                               | ✅       |
| `pathSegment`               | `true`            | bool     | Use match on path segment to pass a valid key.                    | ⚠️        |
| `permissiveMode`            | `false`           | bool     | Dry-run option to allow the request even if no valid was provided | ✅       |
| `removeHeadersOnSuccess`    | `true`            | bool     | If true will remove the header on success.                        | ✅       |
| `internalForwardHeaderName` | `""`              | string   | Optionally forward validated key as header to next middleware.    | ✅       |
| `internalErrorRoute`        | `""`              | string   | On invalid key, forward request to backend at this path instead of 403. | ✅       |
| `exemptPaths`               | `[]`              | []string | Path prefixes that skip authentication (e.g. `/health`, `/metrics`).     | ✅       |
| `keys`                      | `[]`              | []string | Valid keys. Use `env:VAR_NAME` to load a key from an environment variable. | ❌       |

⚠️ - At least one of `authenticationHeader`, `bearerHeader`, `queryParam` or `pathSegment` must be set to `true`.

❌ - Required.

✅ - Is optional and will use the default values if not set.

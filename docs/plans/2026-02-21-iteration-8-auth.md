# Iteration 8: Authentication & Authorization

## Scope
OAuth2/OIDC login, cookie-based sessions, RBAC middleware. Auth is optional — disabled by default.

## Architecture

### Backend
- `internal/auth/oidc.go` — OIDC provider wrapper (go-oidc + oauth2)
- `internal/auth/session.go` — HMAC-signed cookie sessions
- `internal/auth/rbac.go` — Role-based access control engine
- `internal/api/middleware/auth.go` — Auth + RBAC middleware
- `internal/api/handlers/auth.go` — Login/callback/logout/me/status endpoints

### Auth Flow
1. User hits UI → checks /api/v1/auth/me
2. If not authenticated → redirect to /api/v1/auth/login
3. Login redirects to OIDC provider
4. After auth → callback exchanges code for tokens
5. Session created via signed cookie
6. RBAC checked per request

### API Endpoints
| Method | Path | Description |
|--------|------|-------------|
| GET | `/auth/login` | Redirect to OIDC provider |
| GET | `/auth/callback` | OAuth2 callback |
| POST | `/auth/logout` | Clear session |
| GET | `/auth/me` | Current user info |
| GET | `/auth/status` | Auth config status |

### Config
```yaml
auth:
  enabled: true
  type: oidc
  oidc:
    issuer: https://keycloak.example.com/realms/kafka
    client-id: kafka-ui
    client-secret: ${OIDC_CLIENT_SECRET}
    scopes: [openid, profile, email]
    redirect-url: http://localhost:8080/api/v1/auth/callback
  session:
    secret: ${SESSION_SECRET}
    max-age: 86400
  rbac:
    - role: admin
      clusters: ["*"]
      actions: ["*"]
```

### RBAC Actions
view_topics, create_topics, delete_topics, view_messages, produce_messages, view_consumer_groups, reset_offsets, view_schemas, manage_schemas, view_connectors, manage_connectors, execute_ksql, view_acls, manage_acls

## Testing
- RBAC engine tests (8)
- Session management tests (5)
- Handler tests (auth status, me endpoint)

## Files
- `internal/auth/oidc.go` — new
- `internal/auth/session.go` — new
- `internal/auth/rbac.go` — new
- `internal/auth/rbac_test.go` — new
- `internal/auth/session_test.go` — new
- `internal/api/middleware/auth.go` — new
- `internal/api/handlers/auth.go` — new
- `internal/config/config.go` — updated (OIDC, Session, RBAC configs)

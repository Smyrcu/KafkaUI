# OIDC Authentication

OpenID Connect (OIDC) is a standard identity layer built on top of OAuth 2.0. KafkaUI supports
any OIDC-compliant provider including Keycloak, Auth0, Okta, Dex, and others. You can configure
multiple providers simultaneously and users can choose which one to log in with.

## Supported Providers

Any identity provider that exposes an OIDC discovery document at
`{issuer}/.well-known/openid-configuration` is compatible. Tested providers:

- **Keycloak** (self-hosted)
- **Auth0**
- **Okta**
- **Dex**
- **Google** (see [google.md](google.md) for the step-by-step guide)
- **GitLab** (see [gitlab.md](gitlab.md) — GitLab is OIDC-compliant)

## How It Works

1. The user clicks "Sign in with \<provider\>" on the KafkaUI login page.
2. KafkaUI redirects the browser to the provider's authorization endpoint with a state parameter.
3. After the user authenticates, the provider redirects back to KafkaUI's callback URL with an
   authorization code.
4. KafkaUI exchanges the code for an ID token and access token, verifies the ID token signature
   against the provider's public keys, and extracts user identity claims.
5. KafkaUI creates or updates the user record, resolves roles, and sets a signed session cookie.

## YAML Configuration

```yaml
auth:
  enabled: true
  types:
    - oidc

  session:
    secret: "${SESSION_SECRET}"
    max-age: 86400

  storage:
    path: "/data/kafkaui-users.db"   # writable path for the SQLite user store

  default-role: viewer   # role assigned when no auto-assignment rule matches

  oidc:
    redirect-url: "https://kafkaui.example.com/auth/callback"

    providers:
      - name: keycloak                    # internal identifier, used in the callback URL
        display-name: "Keycloak"          # shown on the login page
        issuer: "https://auth.example.com/realms/myrealm"
        client-id: "${KEYCLOAK_CLIENT_ID}"
        client-secret: "${KEYCLOAK_CLIENT_SECRET}"
        scopes:
          - openid
          - profile
          - email
```

You can omit `scopes` — KafkaUI defaults to `["openid", "profile", "email"]`.

## Scopes Explained

| Scope     | Purpose                                                                    |
|-----------|----------------------------------------------------------------------------|
| `openid`  | Required. Requests an ID token. Without it the response is plain OAuth 2.0 and KafkaUI cannot authenticate the user. |
| `profile` | Includes `name`, `picture`, and other basic profile fields in the ID token. |
| `email`   | Includes the `email` claim. Required for email-based auto-assignment rules. |

Add provider-specific scopes (e.g., Keycloak's `roles`) only when you need claims beyond these
three.

## Role Extraction from Claims

KafkaUI inspects the following ID token claims in priority order to extract group/role membership
for use in auto-assignment rules:

| Priority | Claim                | Typical provider       |
|----------|----------------------|------------------------|
| 1        | `groups`             | Generic OIDC, Dex, Okta |
| 2        | `realm_access.roles` | Keycloak               |

The extracted values are stored as the user's `Orgs` and can be matched in auto-assignment rules
via `gitlab-groups` or custom matching. They do **not** automatically become KafkaUI roles — you
still need an `auto-assignment` rule or manual role assignment via the UI.

## Multiple Providers Simultaneously

List as many providers as needed under `oidc.providers`. Each must have a unique `name`. The
callback URL is shared — KafkaUI disambiguates providers by the `state` parameter it generates at
the start of each login flow.

```yaml
# Under auth: section in kafkaui.yaml
auth:
  oidc:
    redirect-url: "https://kafkaui.example.com/auth/callback"

    providers:
      - name: keycloak
        display-name: "Keycloak (Internal)"
        issuer: "https://auth.internal/realms/staff"
        client-id: "${KEYCLOAK_CLIENT_ID}"
        client-secret: "${KEYCLOAK_CLIENT_SECRET}"

      - name: auth0
        display-name: "Auth0 (Partners)"
        issuer: "https://myapp.auth0.com/"
        client-id: "${AUTH0_CLIENT_ID}"
        client-secret: "${AUTH0_CLIENT_SECRET}"
```

Both providers appear as separate buttons on the login page.

## Auto-Assignment Example (Keycloak Roles)

Keycloak puts realm roles in `realm_access.roles`. KafkaUI extracts them into the user's `Orgs`
field. Match them with `gitlab-groups` (reused for generic OIDC group claims):

```yaml
auth:
  auto-assignment:
    - role: viewer
      match:
        authenticated: true          # any logged-in user gets viewer

    - role: editor
      match:
        gitlab-groups:               # matches realm_access.roles or groups claim
          - kafka-editors

    - role: admin
      match:
        emails:
          - ops-lead@example.com
```

## Troubleshooting

**"redirect_uri_mismatch" from the provider**

- The `redirect-url` in KafkaUI config must exactly match one of the authorized redirect URIs
  registered in the provider's application settings (scheme, host, port, and path must all match).
- Trailing slashes matter. `https://kafkaui.example.com/auth/callback` and
  `https://kafkaui.example.com/auth/callback/` are different URIs.

**"verifying ID token" error at login**

- The provider's clock and the KafkaUI server's clock must be within a few seconds of each other.
  Check NTP synchronization on the server running KafkaUI.
- Confirm the `issuer` URL in the config matches the `iss` claim in the token exactly (no trailing
  slash discrepancy).

**No groups/roles extracted from the token**

- Check what claims the provider actually includes in the ID token by decoding a sample token at
  [jwt.io](https://jwt.io). Look for `groups`, `realm_access`, or a custom claim.
- For Keycloak, ensure the "Add to ID token" mapper is enabled for `realm roles` in the client
  configuration.
- Auth0 requires a custom Action or Rule to add group claims to the token.

**Users not getting the expected role**

- Check the `auto-assignment` rules and remember that all conditions within a `match` block are
  evaluated with AND logic. A user must satisfy every listed condition to match the rule.
- Verify `default-role` is set if you want unauthenticated or unmatched users to have a fallback
  role.
- Roles manually assigned via the UI (/settings/users) take priority over auto-assignment rules.

## Deployment

### SQLite User Store

KafkaUI stores user records and manually-assigned role overrides in a SQLite database. Set
`auth.storage.path` to a writable path appropriate for your environment:

```yaml
auth:
  storage:
    path: "/data/kafkaui-users.db"
```

### Docker

```bash
docker run -d \
  -p 8080:8080 \
  -v kafkaui-data:/data \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -e KEYCLOAK_CLIENT_ID="kafkaui" \
  -e KEYCLOAK_CLIENT_SECRET="your_client_secret" \
  -v /path/to/kafkaui.yaml:/etc/kafkaui/config.yaml \
  ghcr.io/your-org/kafkaui:latest \
  --config /etc/kafkaui/config.yaml
```

Set `auth.storage.path: /data/kafkaui-users.db` in the config to persist the user store across
container restarts.

### Helm

Store credentials in a Kubernetes Secret (one secret per OIDC provider, or combine them):

```bash
kubectl create secret generic kafkaui-oidc \
  --from-literal=SESSION_SECRET="$(openssl rand -hex 32)" \
  --from-literal=KEYCLOAK_CLIENT_ID="kafkaui" \
  --from-literal=KEYCLOAK_CLIENT_SECRET="your_client_secret"
```

In `values.yaml`:

```yaml
env:
  - name: SESSION_SECRET
    valueFrom:
      secretKeyRef:
        name: kafkaui-oidc
        key: SESSION_SECRET
  - name: KEYCLOAK_CLIENT_ID
    valueFrom:
      secretKeyRef:
        name: kafkaui-oidc
        key: KEYCLOAK_CLIENT_ID
  - name: KEYCLOAK_CLIENT_SECRET
    valueFrom:
      secretKeyRef:
        name: kafkaui-oidc
        key: KEYCLOAK_CLIENT_SECRET

persistence:
  enabled: true
  mountPath: /data
  size: 1Gi
```

Set `auth.storage.path: /data/kafkaui-users.db` in the KafkaUI config. If the pod has
`readOnlyRootFilesystem: true`, this persistent volume is required.

### Binary

```bash
SESSION_SECRET="$(openssl rand -hex 32)" \
KEYCLOAK_CLIENT_ID="kafkaui" \
KEYCLOAK_CLIENT_SECRET="your_client_secret" \
./kafkaui --config kafkaui.yaml
```

## Security

- **Use HTTPS in production.** OIDC providers typically reject `http://` redirect URIs for
  non-localhost addresses. Configure TLS at the reverse proxy level and register only `https://`
  redirect URIs with the provider.
- **Generate a strong session secret:**
  ```bash
  openssl rand -hex 32
  ```
  Store the result in `SESSION_SECRET` and reference it as `${SESSION_SECRET}` in the config.
- **The redirect URI must match exactly.** Register `https://kafkaui.example.com/auth/callback` with
  the identity provider and use the same URL in `auth.oidc.redirect-url`. Trailing slashes and
  scheme differences cause mismatches.

## See Also

- [Roles and Permissions](roles-and-permissions.md)
- [Google OIDC](google.md)
- [GitLab OIDC](gitlab.md)

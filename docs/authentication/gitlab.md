# GitLab Authentication

GitLab is OIDC-compliant, so it is configured under the `oidc` section. KafkaUI uses the standard
OIDC discovery document to obtain endpoints and verify tokens. Both GitLab.com and self-hosted
GitLab instances are supported.

## Prerequisites

- Admin access to the GitLab instance (for instance-level applications), or a regular account (for
  user-level applications).
- GitLab 11.0 or later (OIDC discovery support).

## Step-by-Step: Create a GitLab Application

### Instance-Level Application (recommended for organizations)

1. Open your GitLab instance and navigate to **Admin Area → Applications** (the hammer icon in the
   top navigation, then **Applications** in the left sidebar).
2. Click **New application**.
3. Fill in:
   - **Name**: KafkaUI
   - **Redirect URI**:
     ```
     https://kafkaui.example.com/auth/callback
     ```
   - **Trusted**: check this box to skip the per-user OAuth consent screen.
   - **Confidential**: leave checked (KafkaUI uses a server-side secret).
4. Under **Scopes**, check:
   - `openid`
   - `profile`
   - `email`
   - `read_api` (needed to read group memberships for auto-assignment)
5. Click **Save application**.
6. Copy the **Application ID** (this is the client ID) and **Secret**.

### User-Level Application

If you do not have admin access, create the application under **User Settings → Applications**.
User-level applications can still use OIDC but cannot read group memberships without `read_api`.

## Required Scopes

| Scope      | Purpose                                                                                   |
|------------|-------------------------------------------------------------------------------------------|
| `openid`   | Required. Issues an OIDC ID token.                                                        |
| `profile`  | Includes `name` and `picture` in the ID token.                                            |
| `email`    | Includes the `email` claim. Required for email-based auto-assignment rules.               |
| `read_api` | Allows KafkaUI to read the user's group memberships via the GitLab API, which are included in the `groups` claim of the ID token. |

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

  default-role: viewer               # fallback role when no auto-assignment rule matches

  oidc:
    redirect-url: "https://kafkaui.example.com/auth/callback"

    providers:
      - name: gitlab
        display-name: "GitLab"
        # For gitlab.com:
        issuer: "https://gitlab.com"
        # For self-hosted: issuer: "https://gitlab.internal.example.com"
        client-id: "${GITLAB_CLIENT_ID}"
        client-secret: "${GITLAB_CLIENT_SECRET}"
        scopes:
          - openid
          - profile
          - email
          - read_api
```

## Auto-Assignment by GitLab Group

GitLab includes group membership in the `groups` claim of the ID token when `read_api` is granted.
KafkaUI maps these to the `Orgs` field and matches them with `gitlab-groups` rules.

```yaml
auth:
  auto-assignment:
    # Any authenticated GitLab user gets viewer
    - role: viewer
      match:
        authenticated: true

    # Members of the kafka-editors group get editor
    - role: editor
      match:
        gitlab-groups:
          - kafka-editors          # top-level group name

    # Members of a subgroup
    - role: editor
      match:
        gitlab-groups:
          - infrastructure/kafka   # group/subgroup path

    # Specific admin by email
    - role: admin
      match:
        emails:
          - ops-lead@example.com
```

Group paths are case-sensitive and must match the full path as seen in the GitLab URL
(`gitlab.example.com/infrastructure/kafka` → use `infrastructure/kafka`).

## Troubleshooting

**"Could not create OIDC provider" at startup**

- KafkaUI fetches the discovery document from `{issuer}/.well-known/openid-configuration` at
  startup. If the issuer URL is wrong or unreachable the server will fail to start.
- For self-hosted GitLab, use the base URL without a trailing slash, e.g.
  `https://gitlab.internal.example.com`.
- GitLab.com issuer is exactly `https://gitlab.com` (no trailing slash).

**"redirect_uri_mismatch"**

- The `redirect-url` in the KafkaUI config must exactly match the **Redirect URI** in the GitLab
  application settings.

**Groups claim is empty / `gitlab-groups` rules not matching**

- `read_api` scope must be requested. Without it, GitLab omits the `groups` claim.
- Ensure the user is an active member (not a pending invitation) of the group.
- Nested group membership is included only if the application is configured at the instance level
  and the user has at least Guest access on the group.

**Self-hosted GitLab — ID token verification fails**

- Confirm the issuer URL in the config exactly matches the `iss` claim in the token. Decode a
  sample token at [jwt.io](https://jwt.io) to verify.
- Ensure TLS certificates on the GitLab server are valid and trusted by the system where KafkaUI
  runs. Custom CA certificates must be installed in the system trust store or provided via
  `SSL_CERT_FILE` / `SSL_CERT_DIR` environment variables.

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
  -e GITLAB_CLIENT_ID="your_application_id" \
  -e GITLAB_CLIENT_SECRET="your_secret" \
  -v /path/to/kafkaui.yaml:/etc/kafkaui/config.yaml \
  ghcr.io/your-org/kafkaui:latest \
  --config /etc/kafkaui/config.yaml
```

Set `auth.storage.path: /data/kafkaui-users.db` in the config to persist the user store across
container restarts.

### Helm

Store credentials in a Kubernetes Secret:

```bash
kubectl create secret generic kafkaui-gitlab \
  --from-literal=SESSION_SECRET="$(openssl rand -hex 32)" \
  --from-literal=GITLAB_CLIENT_ID="your_application_id" \
  --from-literal=GITLAB_CLIENT_SECRET="your_secret"
```

In `values.yaml`:

```yaml
env:
  - name: SESSION_SECRET
    valueFrom:
      secretKeyRef:
        name: kafkaui-gitlab
        key: SESSION_SECRET
  - name: GITLAB_CLIENT_ID
    valueFrom:
      secretKeyRef:
        name: kafkaui-gitlab
        key: GITLAB_CLIENT_ID
  - name: GITLAB_CLIENT_SECRET
    valueFrom:
      secretKeyRef:
        name: kafkaui-gitlab
        key: GITLAB_CLIENT_SECRET

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
GITLAB_CLIENT_ID="your_application_id" \
GITLAB_CLIENT_SECRET="your_secret" \
./kafkaui --config kafkaui.yaml
```

## Security

- **Use HTTPS in production.** The OIDC callback URL must use `https://` — GitLab rejects `http://`
  redirect URIs for non-localhost addresses.
- **Generate a strong session secret:**
  ```bash
  openssl rand -hex 32
  ```
  Store the result in `SESSION_SECRET` and reference it as `${SESSION_SECRET}` in the config.
- **The redirect URI must match exactly.** Register `https://kafkaui.example.com/auth/callback` in
  the GitLab application settings and use the same URL in `auth.oidc.redirect-url`.

## See Also

- [Roles and Permissions](roles-and-permissions.md)

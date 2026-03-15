# GitHub OAuth Authentication

GitHub uses its own OAuth 2.0 flow (not OIDC), so it is configured under the `oauth2` section —
not `oidc`. KafkaUI fetches the user's profile, primary email, organization memberships, and team
memberships from the GitHub API after the OAuth exchange.

## Prerequisites

- A GitHub account with access to create an OAuth App under your personal account or an
  organization.
- Use a **GitHub OAuth App**, not a GitHub App (GitHub Apps use different installation-based flows
  that are not supported).

## Step-by-Step: Create a GitHub OAuth App

### Personal Account

1. Open GitHub and go to **Settings → Developer settings → OAuth Apps**.
2. Click **New OAuth App**.
3. Fill in:
   - **Application name**: KafkaUI (or any name your users will recognise on the consent screen).
   - **Homepage URL**: your KafkaUI URL, e.g. `https://kafkaui.example.com`.
   - **Authorization callback URL**:
     ```
     https://kafkaui.example.com/auth/callback
     ```
4. Click **Register application**.
5. On the app detail page, click **Generate a new client secret**.
6. Copy the **Client ID** and **Client secret**.

### Organization-Owned App

Follow the same steps but start from **Organization Settings → Developer settings → OAuth Apps**.
Organization-owned apps can request access to organization resources without additional per-member
approval workflows.

## Required Scopes

| Scope        | Purpose                                                                          |
|--------------|----------------------------------------------------------------------------------|
| `user:email` | Reads the user's email addresses. Required for email-based auto-assignment rules and to identify the user. GitHub may not include the primary email in the basic user response if it is set to private, so this scope is always needed. |
| `read:org`   | Reads organization and team membership. Required for `github-orgs` and `github-teams` auto-assignment rules. Without it, KafkaUI cannot see which organizations the user belongs to. |

## YAML Configuration

```yaml
auth:
  enabled: true
  types:
    - oauth2

  session:
    secret: "${SESSION_SECRET}"
    max-age: 86400

  storage:
    path: "/data/kafkaui-users.db"   # writable path for the SQLite user store

  default-role: viewer               # fallback role when no auto-assignment rule matches

  oauth2:
    redirect-url: "https://kafkaui.example.com/auth/callback"

    providers:
      - name: github
        display-name: "GitHub"
        client-id: "${GITHUB_CLIENT_ID}"
        client-secret: "${GITHUB_CLIENT_SECRET}"
        scopes:
          - user:email
          - read:org
```

If `scopes` is omitted, KafkaUI defaults to `["user:email", "read:org"]`.

## Auto-Assignment by Organization and Team

```yaml
auth:
  auto-assignment:
    # Any member of the org gets viewer
    - role: viewer
      match:
        github-orgs:
          - my-org

    # Members of the platform team get editor
    - role: editor
      match:
        github-orgs:
          - my-org
        github-teams:
          - my-org/platform-team   # format: org-name/team-slug

    # Specific admins by email
    - role: admin
      match:
        emails:
          - sre-lead@example.com
```

All conditions within a `match` block use AND logic — all must be true for the rule to match. A
user collects roles from all rules that match (OR across rules).

Team slugs are formatted as `org-name/team-slug`. The team slug is the URL-safe version of the
team name visible in the GitHub URL: `github.com/orgs/my-org/teams/platform-team`.

## Troubleshooting

**Organization membership not detected / `github-orgs` rule not matching**

- The `read:org` scope must be granted. If the user authorized the app before this scope was added,
  they must re-authorize. Revoke the app token under **Settings → Applications → Authorized OAuth
  Apps** and log in again.
- If the organization has **third-party application restrictions** enabled, the OAuth App must be
  approved by an organization owner under **Organization Settings → Third-party access**.

**Private email not returned**

- Ensure `user:email` is in the requested scopes. Without it the GitHub API may return an empty
  email list.

**Team membership not detected**

- Team membership is visible to the app only if the user's membership in the team is **public**, or
  if the app has been granted access and the `read:org` scope is approved.
- Verify the team slug by navigating to `github.com/orgs/<org>/teams/<slug>`. The slug is the last
  path segment.

**"Authorization callback URL mismatch"**

- The `redirect-url` in the KafkaUI config must exactly match the **Authorization callback URL**
  registered in the OAuth App settings.

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
  -e GITHUB_CLIENT_ID="Iv1.your_client_id" \
  -e GITHUB_CLIENT_SECRET="your_client_secret" \
  -v /path/to/kafkaui.yaml:/etc/kafkaui/config.yaml \
  ghcr.io/your-org/kafkaui:latest \
  --config /etc/kafkaui/config.yaml
```

Set `auth.storage.path: /data/kafkaui-users.db` in the config to persist the user store across
container restarts.

### Helm

Store credentials in a Kubernetes Secret:

```bash
kubectl create secret generic kafkaui-github \
  --from-literal=SESSION_SECRET="$(openssl rand -hex 32)" \
  --from-literal=GITHUB_CLIENT_ID="Iv1.your_client_id" \
  --from-literal=GITHUB_CLIENT_SECRET="your_client_secret"
```

In `values.yaml`:

```yaml
env:
  - name: SESSION_SECRET
    valueFrom:
      secretKeyRef:
        name: kafkaui-github
        key: SESSION_SECRET
  - name: GITHUB_CLIENT_ID
    valueFrom:
      secretKeyRef:
        name: kafkaui-github
        key: GITHUB_CLIENT_ID
  - name: GITHUB_CLIENT_SECRET
    valueFrom:
      secretKeyRef:
        name: kafkaui-github
        key: GITHUB_CLIENT_SECRET

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
GITHUB_CLIENT_ID="Iv1.your_client_id" \
GITHUB_CLIENT_SECRET="your_client_secret" \
./kafkaui --config kafkaui.yaml
```

## Security

- **Use HTTPS in production.** The OAuth callback URL must use `https://` — GitHub rejects `http://`
  redirect URIs for non-localhost addresses.
- **Generate a strong session secret:**
  ```bash
  openssl rand -hex 32
  ```
  Store the result in `SESSION_SECRET` and reference it as `${SESSION_SECRET}` in the config.
- **The redirect URI must match exactly.** Register `https://kafkaui.example.com/auth/callback` in
  the GitHub OAuth App settings and use the same URL in `auth.oauth2.redirect-url`.

## See Also

- [Roles and Permissions](roles-and-permissions.md)

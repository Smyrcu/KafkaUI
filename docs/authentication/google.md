# Google OAuth 2.0 Authentication

Google supports OIDC, so it is configured under the `oidc` section — not `oauth2`. This guide
walks through creating the credentials in Google Cloud Console and wiring them into KafkaUI.

## Prerequisites

- A Google Cloud project (create one at [console.cloud.google.com](https://console.cloud.google.com) if you do not have one).
- Billing does not need to be enabled for OAuth.

## Step-by-Step: Create OAuth 2.0 Credentials

### 1. Configure the OAuth Consent Screen

1. In the Cloud Console, open **APIs & Services → OAuth consent screen**.
2. Choose **Internal** if all users are in your Google Workspace organization, or **External** for
   any Google account.
3. Fill in **App name**, **User support email**, and **Developer contact email**.
4. Under **Scopes**, add the following (or leave default and rely on runtime scope requests):
   - `.../auth/userinfo.email`
   - `.../auth/userinfo.profile`
   - `openid`
5. Save and continue through the remaining screens. Publishing is not required for Internal apps.
   For External apps in testing mode, add the accounts that should be able to log in under
   **Test users**.

### 2. Create OAuth 2.0 Client Credentials

1. Open **APIs & Services → Credentials**.
2. Click **Create Credentials → OAuth client ID**.
3. Set **Application type** to **Web application**.
4. Give it a name (e.g., "KafkaUI").
5. Under **Authorized redirect URIs**, add:
   ```
   https://kafkaui.example.com/auth/callback
   ```
   Use the exact same URL you will put in the KafkaUI config. HTTP is only acceptable for
   `localhost` during development.
6. Click **Create**. Copy the **Client ID** and **Client Secret**.

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
      - name: google
        display-name: "Google"
        issuer: "https://accounts.google.com"
        client-id: "${GOOGLE_CLIENT_ID}"
        client-secret: "${GOOGLE_CLIENT_SECRET}"
        scopes:
          - openid
          - profile
          - email
```

Google's OIDC issuer is always `https://accounts.google.com` — do not change it.

## Auto-Assignment by Domain

Restrict access to users from a specific Google Workspace domain:

```yaml
auth:
  auto-assignment:
    - role: viewer
      match:
        email-domains:
          - "@example.com"      # only @example.com Google accounts

    - role: admin
      match:
        emails:
          - sre-lead@example.com
```

## Troubleshooting

**"Error 400: redirect_uri_mismatch"**

- The redirect URI in the KafkaUI config must exactly match one of the URIs listed in the Cloud
  Console under **Authorized redirect URIs** — character for character, including scheme, port, and
  trailing slash (or lack thereof).
- Propagation after adding a URI can take up to five minutes.

**"Access blocked: app has not completed the Google verification process"**

- This appears for External apps when the app is in **Testing** mode and the account is not in the
  test users list. Add the account under **OAuth consent screen → Test users**, or publish the app.
- Internal apps do not require verification but are restricted to accounts within your Google
  Workspace organization.

**Users from outside the organization can log in**

- Set the consent screen to **Internal**, or use `email-domains` auto-assignment rules to restrict
  which accounts can receive a role. Without a role, users can authenticate but cannot access any
  resources.

**ID token cannot be verified**

- Ensure the server running KafkaUI has accurate system time. Google's token verifier checks the
  `exp` and `iat` claims against the current time with a small tolerance.

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
  -e GOOGLE_CLIENT_ID="your_client_id.apps.googleusercontent.com" \
  -e GOOGLE_CLIENT_SECRET="your_client_secret" \
  -v /path/to/kafkaui.yaml:/etc/kafkaui/config.yaml \
  ghcr.io/your-org/kafkaui:latest \
  --config /etc/kafkaui/config.yaml
```

Set `auth.storage.path: /data/kafkaui-users.db` in the config to persist the user store across
container restarts.

### Helm

Store credentials in a Kubernetes Secret:

```bash
kubectl create secret generic kafkaui-google \
  --from-literal=SESSION_SECRET="$(openssl rand -hex 32)" \
  --from-literal=GOOGLE_CLIENT_ID="your_client_id.apps.googleusercontent.com" \
  --from-literal=GOOGLE_CLIENT_SECRET="your_client_secret"
```

In `values.yaml`:

```yaml
env:
  - name: SESSION_SECRET
    valueFrom:
      secretKeyRef:
        name: kafkaui-google
        key: SESSION_SECRET
  - name: GOOGLE_CLIENT_ID
    valueFrom:
      secretKeyRef:
        name: kafkaui-google
        key: GOOGLE_CLIENT_ID
  - name: GOOGLE_CLIENT_SECRET
    valueFrom:
      secretKeyRef:
        name: kafkaui-google
        key: GOOGLE_CLIENT_SECRET

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
GOOGLE_CLIENT_ID="your_client_id.apps.googleusercontent.com" \
GOOGLE_CLIENT_SECRET="your_client_secret" \
./kafkaui --config kafkaui.yaml
```

## Security

- **Use HTTPS in production.** Google rejects `http://` redirect URIs for any non-localhost address.
  Configure TLS at the reverse proxy level and register only `https://` URIs in the Cloud Console.
- **Generate a strong session secret:**
  ```bash
  openssl rand -hex 32
  ```
  Store the result in `SESSION_SECRET` and reference it as `${SESSION_SECRET}` in the config.
- **The redirect URI must match exactly.** Register `https://kafkaui.example.com/auth/callback` in
  **Authorized redirect URIs** and use the same URL in `auth.oidc.redirect-url`. Even a trailing
  slash difference causes an error.

## See Also

- [Roles and Permissions](roles-and-permissions.md)

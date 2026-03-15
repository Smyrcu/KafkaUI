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

  default-role: viewer
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

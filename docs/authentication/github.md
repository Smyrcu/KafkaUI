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

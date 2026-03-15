# Roles and Permissions

KafkaUI uses a role-based access control (RBAC) system. Each user holds one or more roles, and
each role is granted a set of actions on specific clusters. Roles are resolved at login time and
attached to the session.

## Role Concepts

A **role** is a named string (e.g., `viewer`, `editor`, `admin`). It has no inherent meaning until
bound to actions via an RBAC rule. The binding lives in `rbac.rules`.

A **role group** is a reusable alias for a list of actions (or other role groups) defined in
`rbac.role-groups`. Using role groups keeps rules DRY — you define the action set once and
reference it by name.

An **RBAC rule** connects a role name to a list of actions on a list of clusters:

```yaml
# Under auth: section in kafkaui.yaml
auth:
  rbac:
    rules:
      - role: viewer
        clusters: ["*"]      # wildcard = all clusters
        actions: [view]      # "view" expands via role-groups
```

## Built-in Role Groups

KafkaUI ships without hard-coded roles — you define them yourself in `rbac.role-groups`. The
recommended starting point uses three groups that nest into each other:

| Group  | Contains                                   |
|--------|--------------------------------------------|
| `view` | All `view_*` actions (read-only access)    |
| `edit` | Everything in `view` + all mutating actions |
| `admin`| Everything in `edit` + `manage_users` + `manage_clusters` |

```yaml
# Under auth: section in kafkaui.yaml
auth:
  rbac:
    role-groups:
      view:
        - view_dashboard
        - view_brokers
        - view_topics
        - view_messages
        - view_consumer_groups
        - view_schemas
        - view_connectors
        - view_acls
        - view_ksql
        - view_kafka_users

      edit:
        - view                      # includes all view_* actions
        - create_topics
        - delete_topics
        - produce_messages
        - create_schemas
        - delete_schemas
        - manage_connectors
        - reset_consumer_groups
        - create_acls
        - delete_acls
        - execute_ksql
        - manage_kafka_users

      admin:
        - edit                      # includes view + all mutating actions
        - manage_users
        - manage_clusters
```

Role groups can reference other role groups recursively. Circular references are detected and
skipped.

## Action Catalog

| Action                  | What It Controls                                               |
|-------------------------|----------------------------------------------------------------|
| `view_dashboard`        | View the cluster overview dashboard                            |
| `view_brokers`          | List brokers and their configuration                           |
| `view_topics`           | List topics, view topic details and configuration              |
| `view_messages`         | Browse and search topic messages                               |
| `view_consumer_groups`  | List consumer groups and their lag                             |
| `view_schemas`          | List and view Schema Registry schemas                          |
| `view_connectors`       | List and view Kafka Connect connectors                         |
| `view_acls`             | List ACL entries                                               |
| `view_ksql`             | Access the KSQL query editor                                   |
| `view_kafka_users`      | List Kafka users (SCRAM credentials)                           |
| `create_topics`         | Create new topics                                              |
| `delete_topics`         | Delete existing topics                                         |
| `produce_messages`      | Produce messages to topics                                     |
| `create_schemas`        | Register new Schema Registry schemas                           |
| `delete_schemas`        | Delete Schema Registry schemas                                 |
| `manage_connectors`     | Create, update, pause, resume, restart, and delete connectors  |
| `reset_consumer_groups` | Reset consumer group offsets                                   |
| `create_acls`           | Create ACL entries                                             |
| `delete_acls`           | Delete ACL entries                                             |
| `execute_ksql`          | Execute KSQL queries                                           |
| `manage_kafka_users`    | Create and delete Kafka SCRAM users                            |
| `manage_users`          | Manage KafkaUI user accounts and role assignments via the UI   |
| `manage_clusters`       | Add, edit, and remove cluster connections via the UI           |

Use `"*"` in an `actions` list to grant all current and future actions without listing them
individually.

## RBAC Rules

```yaml
# Under auth: section in kafkaui.yaml
auth:
  rbac:
    rules:
      - role: viewer
        clusters: ["*"]
        actions: [view]

      - role: editor
        clusters: ["*"]
        actions: [edit]

      - role: prod-readonly
        clusters: ["prod-eu", "prod-us"]   # restrict to specific clusters
        actions: [view]

      - role: admin
        clusters: ["*"]
        actions: ["*"]                     # wildcard grants all actions
```

`clusters` accepts a list of cluster names as defined in the top-level `clusters` config, or `"*"`
for all clusters. A role can appear in multiple rules with different cluster scopes.

## Role Resolution Priority

KafkaUI determines a user's effective roles using the following priority order:

1. **Manual assignment** — roles set by an admin via the UI (/settings/users). Highest priority;
   overrides everything else.
2. **Auto-assignment rules** — roles computed from the user's identity (email, org, teams). See the
   next section.
3. **Default role** — the `default-role` value in the `auth` config block. Applied when neither of
   the above yields any roles.

## Auto-Assignment Rules

Auto-assignment automatically grants roles to users based on their identity attributes at login.
This avoids manual role assignment for large teams.

### Logic

- **AND within a `match` block** — all listed conditions must be true for the rule to fire.
- **OR across rules** — a user collects roles from every rule that matches. Multiple roles can be
  accumulated.

### Available Match Conditions

| Field           | Applies to                      | Description                                   |
|-----------------|---------------------------------|-----------------------------------------------|
| `authenticated` | Any provider                    | True for any user who has successfully logged in |
| `emails`        | Any provider                    | Exact email address match (case-insensitive)  |
| `email-domains` | Any provider                    | Suffix match on the email address             |
| `github-orgs`   | GitHub OAuth2                   | User is a member of the listed organizations  |
| `github-teams`  | GitHub OAuth2                   | User is a member of the listed teams (`org/team-slug`) |
| `gitlab-groups` | GitLab OIDC                     | User is a member of the listed groups (also matches OIDC `groups` claim) |

### Examples

**All authenticated users get viewer, ops team gets editor, one person gets admin:**

```yaml
auth:
  auto-assignment:
    - role: viewer
      match:
        authenticated: true

    - role: editor
      match:
        github-orgs:
          - my-org
        github-teams:
          - my-org/ops-team      # AND: must be in the org AND in the team

    - role: admin
      match:
        emails:
          - alice@example.com
```

**Domain-restricted access with GitLab groups:**

```yaml
auth:
  auto-assignment:
    - role: viewer
      match:
        email-domains:
          - "@example.com"

    - role: editor
      match:
        email-domains:
          - "@example.com"
        gitlab-groups:
          - platform/kafka
```

## Admin User Management via the UI

Admins (users with the `manage_users` action) can override auto-assignment for individual users
through the UI at **/settings/users**. Manually assigned roles take precedence over all
auto-assignment rules and the default role.

Use this for:
- Granting elevated access to a specific person without changing group memberships.
- Revoking access for a compromised account without touching the config.
- Temporary access that should not be codified in the YAML config.

## Default Role

```yaml
auth:
  default-role: viewer
```

When a user logs in and no auto-assignment rule fires (and no admin has manually set their roles),
they receive `default-role`. Set it to an empty string to deny access to unrecognized users:

```yaml
auth:
  default-role: ""   # users without a matching rule have no roles and cannot access anything
```

## SQLite User Store (`auth.storage.path`)

When authentication is enabled, KafkaUI creates a SQLite database to persist user records and
manually-assigned roles. The default file name is `kafkaui-users.db` created in the **current
working directory** of the process.

```yaml
auth:
  storage:
    path: "/data/kafkaui-users.db"   # absolute path recommended for predictability
```

### Important deployment notes

- The directory containing the database file must be **writable** by the KafkaUI process.
- If the file does not exist, KafkaUI creates it on first startup.
- **Kubernetes with `readOnlyRootFilesystem: true`**: the default CWD path will fail. Mount a
  writable `PersistentVolumeClaim` and set `auth.storage.path` to a path on that volume (e.g.
  `/data/kafkaui-users.db`).
- **Docker**: mount a host directory or named volume to preserve the database across container
  restarts. Without a volume the database is lost when the container is removed.
- **Binary**: the database is created in whatever directory you launch the binary from. Use an
  absolute path in the config to make the location explicit and stable.

## Complete Example Configurations

### Read-Only Team

A team that should see everything but change nothing:

```yaml
auth:
  enabled: true
  types: [oidc]

  default-role: ""

  auto-assignment:
    - role: readonly
      match:
        email-domains:
          - "@analytics.example.com"

  rbac:
    role-groups:
      view:
        - view_dashboard
        - view_brokers
        - view_topics
        - view_messages
        - view_consumer_groups
        - view_schemas
        - view_connectors
        - view_acls
        - view_ksql
        - view_kafka_users

    rules:
      - role: readonly
        clusters: ["*"]
        actions: [view]
```

### Ops Team

Engineers who manage topics and connectors but cannot touch user management:

```yaml
auth:
  enabled: true
  types: [oauth2]   # GitHub

  default-role: ""

  auto-assignment:
    - role: ops
      match:
        github-orgs:
          - mycompany
        github-teams:
          - mycompany/platform-engineering

  rbac:
    role-groups:
      view:
        - view_dashboard
        - view_brokers
        - view_topics
        - view_messages
        - view_consumer_groups
        - view_schemas
        - view_connectors
        - view_acls
        - view_ksql
        - view_kafka_users
      edit:
        - view
        - create_topics
        - delete_topics
        - produce_messages
        - create_schemas
        - delete_schemas
        - manage_connectors
        - reset_consumer_groups
        - create_acls
        - delete_acls
        - execute_ksql
        - manage_kafka_users

    rules:
      - role: ops
        clusters: ["*"]
        actions: [edit]
```

### Full Admin

A superuser that can do everything including user and cluster management:

```yaml
auth:
  enabled: true
  types: [oidc]

  default-role: ""

  rbac:
    role-groups:
      view:
        - view_dashboard
        - view_brokers
        - view_topics
        - view_messages
        - view_consumer_groups
        - view_schemas
        - view_connectors
        - view_acls
        - view_ksql
        - view_kafka_users
      edit:
        - view
        - create_topics
        - delete_topics
        - produce_messages
        - create_schemas
        - delete_schemas
        - manage_connectors
        - reset_consumer_groups
        - create_acls
        - delete_acls
        - execute_ksql
        - manage_kafka_users
      admin:
        - edit
        - manage_users
        - manage_clusters

    rules:
      - role: superadmin
        clusters: ["*"]
        actions: ["*"]      # wildcard: current and future actions
```

## Security Checklist

- **Use HTTPS in production.** Session cookies are signed but not encrypted. Transmitting them over
  plain HTTP allows session hijacking. Configure TLS at the reverse proxy level and set
  `redirect-url` to an `https://` URL.
- **Generate a strong session secret.** The session cookie is HMAC-signed with `auth.session.secret`.
  Use at least 32 random bytes:
  ```bash
  openssl rand -hex 32
  ```
  Store the result in an environment variable and reference it as `${SESSION_SECRET}` in the config.
  Never commit the raw secret to source control.
- **Rotate the secret when compromised.** Changing the secret immediately invalidates all existing
  sessions, forcing all users to log in again.

## See Also

- [Basic Authentication](basic-auth.md)
- [GitHub OAuth](github.md)
- [GitLab OIDC](gitlab.md)
- [Google OIDC](google.md)
- [Generic OIDC](oidc.md)

# Basic Authentication

Basic auth lets you define a fixed list of users with bcrypt-hashed passwords directly in the
configuration file. It is the simplest way to add authentication to KafkaUI and requires no
external identity provider.

## How It Works

When a user submits their username and password, KafkaUI compares the password against the
bcrypt hash stored in the config. Roles are assigned directly to each user entry. Sessions are
maintained via a signed cookie so the user is not re-prompted on every page.

## Generating a Bcrypt Hash

Use `htpasswd` from the Apache httpd tools package to generate a cost-10 bcrypt hash:

```bash
htpasswd -nbBC 10 "" yourpassword | cut -d: -f2
```

The output will look like:

```
$2y$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234
```

Paste that entire string (including the `$2y$...` prefix) as the `password` value in the config.

> **Note:** KafkaUI accepts both `$2a$` and `$2y$` bcrypt prefixes. Do not strip the prefix.

## Configuration Example

```yaml
auth:
  enabled: true
  types:
    - basic

  session:
    secret: "${SESSION_SECRET}"   # at least 32 random bytes; keep this secret
    max-age: 86400                 # session lifetime in seconds (86400 = 24 h)

  basic:
    rate-limit:
      max-attempts: 5             # failed attempts allowed before lockout
      window-seconds: 300         # lockout window in seconds (300 = 5 min)

    users:
      - username: alice
        password: "$2y$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"
        roles:
          - viewer

      - username: bob
        password: "$2y$10$zyxwvutsrqponmlkjihgfeedcbaZYXWVUTSRQPONMLKJIHGFEDCBA"
        roles:
          - editor

      - username: admin
        password: "${ADMIN_PASSWORD_HASH}"   # can also come from an env variable
        roles:
          - admin

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
      - role: viewer
        clusters: ["*"]
        actions: [view]

      - role: editor
        clusters: ["*"]
        actions: [edit]

      - role: admin
        clusters: ["*"]
        actions: ["*"]
```

## Rate Limiting

The rate limiter tracks failed login attempts **per IP address** using a fixed-window counter.

| Field            | Type | Default | Description                                    |
|------------------|------|---------|------------------------------------------------|
| `max-attempts`   | int  | —       | Number of failed attempts allowed in the window |
| `window-seconds` | int  | —       | Window duration in seconds; resets after expiry |

When a client exceeds `max-attempts` within the window it receives an HTTP 429 response. The
counter resets automatically after `window-seconds` have elapsed. There is no manual unlock
mechanism; you must wait for the window to expire.

Recommended production values: `max-attempts: 5`, `window-seconds: 300`.

## Session Settings

| Field     | Type   | Description                                                          |
|-----------|--------|----------------------------------------------------------------------|
| `secret`  | string | HMAC key used to sign session cookies. Must be at least 32 bytes. Use `${ENV_VAR}` to avoid storing the secret in the config file. |
| `max-age` | int    | Cookie lifetime in seconds. After this period the session is invalid and the user must log in again. |

## Troubleshooting

**"invalid credentials" even with the correct password**

- Make sure the full hash string is in the config, including the `$2y$10$` (or `$2a$10$`) prefix.
- YAML may interpret the `$` sign if the value is unquoted. Always wrap the hash in double quotes.
- Confirm there are no trailing spaces or newline characters after the hash.
- If the hash was generated with cost other than 10, it still works — cost is embedded in the hash.

**"too many attempts" / HTTP 429**

- The IP has exceeded `max-attempts` failed logins within the `window-seconds` window.
- Wait for the window to expire, or reduce `max-attempts` / increase `window-seconds` in the config.
- If running behind a reverse proxy, make sure the proxy forwards the real client IP via
  `X-Forwarded-For`; otherwise all requests appear to come from the proxy's address and a single
  bad actor can lock out everyone.

**Session expires immediately**

- Check that `max-age` is a positive integer (seconds, not milliseconds).
- Ensure `secret` is stable across restarts. If the secret changes, all existing sessions are
  invalidated.

**User has wrong permissions**

- Verify the role names assigned in `basic.users[].roles` match exactly the role names in `rbac.rules[].role`.
- Role names are case-sensitive.

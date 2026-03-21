# Tier 2 Features Design Spec

**Date:** 2026-03-21
**Scope:** CEL message filters (remaining work), Custom SerDe, LDAP auth, Configuration Wizard
**Approach:** Full project (DDD, Clean Architecture, SOLID)
**Versioning:** Each feature = patch bump (0.13.1, 0.13.2, ...), 0.14.0 after Tier 2 review
**Workflow:** Isolated git worktrees per feature, fix/ branches, merge to main

---

## 1. CEL Message Filters (90% complete — remaining work only)

### Current State

CEL filtering is already implemented and functional:
- `internal/cel/filter.go` — `NewFilter()`, `Match()`, `parseValue()` with 12 test cases
- `internal/api/handlers/message.go` — CEL filter compilation, 5x over-fetch, post-masking application
- `internal/api/ws/livetail.go` — CEL filter in live tail via query parameter
- `frontend/src/pages/TopicMessagesPage.tsx` — filter input UI with help text
- `go.mod` already has `github.com/google/cel-go v0.27.0`

### Remaining Work

Only one piece is missing: a **validation endpoint** for frontend UX.

**New endpoint**: `POST /api/v1/cel/validate`
- Protected by auth middleware + `view_messages` RBAC action
- Request: `{"expression": "value.status == \"FAILED\""}`
- Response: `{"valid": true}` or `{"valid": false, "error": "syntax error at position 5: ..."}`
- Expression length limit: 1000 characters (DoS prevention)

### CEL Environment (existing, for reference)

| Variable | Type | Description |
|----------|------|-------------|
| `key` | string | Message key (UTF-8) |
| `value` | dynamic | JSON object if parseable, string otherwise |
| `headers` | map(string,string) | Message headers |
| `partition` | int | Partition number |
| `offset` | int | Message offset |
| `timestamp` | timestamp | CEL timestamp type (not string) |

### Files (remaining)

| File | Purpose |
|------|---------|
| `internal/api/handlers/cel.go` | `/cel/validate` endpoint |
| `internal/api/router.go` | Wire route under auth+RBAC |

### SerDe Integration Note

When Custom SerDe (feature #2) is added, the deserialized output will replace the raw value in the CEL environment. The existing `parseValue()` in `filter.go` re-parses JSON from strings, so SerDe's pretty-printed JSON output will work transparently (double-parse is minor CPU cost, correctness preserved).

---

## 2. Custom SerDe Plugin System

### Purpose

Deserialize Kafka message keys and values from binary formats (Avro, Protobuf) into human-readable JSON for display in the UI. Without this, Avro/Protobuf topics show garbage bytes.

### Architecture

```
┌──────────┐     ┌────────────┐     ┌──────────────┐
│ Raw bytes │────▶│  Registry  │────▶│ Deserialized │
│ from Kafka│     │ (ordered)  │     │ string (JSON)│
└──────────┘     └────────────┘     └──────────────┘
                       │
              ┌────────┴────────┐
              │ Try each deser  │
              │ in priority     │
              │ order until one │
              │ succeeds        │
              └─────────────────┘
```

### Deserializer Interface

```go
// Deserializer converts raw Kafka message bytes to a display string.
type Deserializer interface {
    // Name returns the deserializer identifier (e.g., "avro", "protobuf", "json").
    Name() string
    // Detect returns true if this deserializer can handle the given data.
    // Called only when no explicit config forces a specific deserializer.
    Detect(topic string, data []byte, headers map[string]string) bool
    // Deserialize converts raw bytes to a display string.
    Deserialize(topic string, data []byte) (string, error)
}
```

Note: `Detect` receives headers alongside data — some producers set `content-type` or schema headers that aid detection.

### Built-in Deserializers

| Name | Detection | Notes |
|------|-----------|-------|
| `avro` | Confluent wire format: byte `0x00` + 4-byte schema ID (payload >= 5 bytes, schema ID lookup succeeds) | Requires Schema Registry URL |
| `protobuf` | Confluent wire format with protobuf marker byte | Requires Schema Registry URL |
| `json` | `json.Valid(data)` | Pretty-prints with indentation |
| `string` | Always true (fallback) | UTF-8 decode, replace invalid bytes |

### Detection Order (auto mode)

1. Avro — check magic byte `0x00`, verify payload >= 5 bytes, extract schema ID, validate against Schema Registry (cached). False positive guard: if schema lookup fails, skip to next.
2. Protobuf — check Confluent protobuf wire format markers
3. JSON — `json.Valid`
4. String — fallback, always succeeds

### Schema Registry Integration

Avro and Protobuf deserializers need schema lookups by ID. The existing `internal/schema/client.go` does NOT have a `GetSchemaByID` method — only subject-based lookups.

**Required addition**: `GetSchemaByID(ctx context.Context, id int) (string, error)` method on the schema client, calling `GET /schemas/ids/{id}`.

**Schema cache**: `sync.Map` keyed by schema ID. Schemas are immutable in Confluent Schema Registry (a given ID always returns the same schema), so cache entries never expire. This is critical for performance — browsing 500 messages with the same schema must not make 500 HTTP calls.

### Configuration

```yaml
clusters:
  - name: production
    schema-registry:
      url: http://schema-registry:8081  # existing config, reused
    serde:
      default: auto  # auto-detect (default), or force: json, string, avro, protobuf
      rules:
        - topic-pattern: "^avro\\."
          key-deserializer: string
          value-deserializer: avro
        - topic-pattern: ".*"
          value-deserializer: auto
```

Topic patterns use Go `regexp.MatchString` — same flavor as `data-masking.rules[].topic-pattern` for consistency.

### Integration Points

- `internal/kafka/client.go` Browse: deserialize before returning `MessageRecord`
- `internal/api/ws/livetail.go`: deserialize before sending to WebSocket
- `MessageRecord.Value` and `MessageRecord.Key` become the deserialized string
- CEL filter receives the deserialized value (existing `parseValue()` re-parses it)

### Files

| File | Purpose |
|------|---------|
| `internal/serde/deserializer.go` | Interface + registry |
| `internal/serde/avro.go` | Avro deserializer with schema cache |
| `internal/serde/protobuf.go` | Protobuf deserializer with schema cache |
| `internal/serde/json.go` | JSON pretty-printer |
| `internal/serde/string.go` | UTF-8 fallback |
| `internal/serde/registry.go` | Per-cluster deserializer chain |
| `internal/serde/*_test.go` | Tests for each deserializer |
| `internal/schema/client.go` | Add `GetSchemaByID` method |
| `internal/config/config.go` | SerDe config types |

### Dependencies

- `github.com/linkedin/goavro/v2` — Avro schema parsing and deserialization
- `google.golang.org/protobuf` — Protobuf dynamic message deserialization
- Existing `internal/schema/client.go` — extended with `GetSchemaByID`

---

## 3. LDAP Authentication

### Purpose

Allow enterprises with LDAP/Active Directory infrastructure to authenticate users without configuring OIDC. Users log in with username/password, which are verified against the LDAP directory.

### Architecture

```
┌──────────┐     ┌────────────┐     ┌──────────┐     ┌──────────┐
│ Login    │────▶│ Service    │────▶│ User     │────▶│ Extract  │
│ form     │     │ Bind       │     │ Search   │     │ Groups   │
│ user+pass│     │ (svc acct) │     │ (find DN)│     │ → Roles  │
└──────────┘     └────────────┘     └──────────┘     └──────────┘
                                         │
                                    ┌────┴─────┐
                                    │ User Bind│
                                    │ (verify  │
                                    │  password)│
                                    └──────────┘
```

### Authentication Flow

1. User submits username + password via login form
2. Server binds to LDAP as service account (`bind-dn` + `bind-password`)
3. Searches for user: `(&(objectClass=person)(uid={username}))` (configurable filter)
4. If found, attempts bind with user's DN + submitted password
5. If bind succeeds, extracts group memberships
6. Creates `UserIdentity` with email, name, groups from LDAP attributes
7. Groups are mapped to RBAC roles via existing auto-assignment rules
8. Session created (reuses existing session infrastructure)

### Login Handler Integration

The existing `LoginBasic` handler (line 81 of `auth.go`) checks `h.hasType("basic")`. For LDAP:

- **Handler reuse**: `LoginBasic` becomes `LoginCredentials` — accepts both `"basic"` and `"ldap"` types. When `"ldap"` is configured, it dispatches to `LDAPAuthenticator` instead of `BasicAuthenticator`. Both return `*UserIdentity`.
- **Dispatch logic**: if `h.hasType("ldap")` and `h.ldap != nil`, try LDAP first. If `h.hasType("basic")` and `h.basic != nil`, try basic. Both can be active simultaneously (try LDAP, fall back to basic).
- **Frontend**: `LoginPage.tsx` line 22 changes from `status?.types?.includes("basic")` to `hasUsernamePassword` = `types includes "basic" OR "ldap"`. The login form is identical for both.

### Configuration

```yaml
auth:
  enabled: true
  types: ["ldap"]
  ldap:
    url: ldap://ldap.example.com:389
    start-tls: true
    connection-timeout: 10s              # dial timeout, default 10s
    bind-dn: cn=kafkaui,ou=services,dc=example,dc=com
    bind-password: ${LDAP_BIND_PASSWORD}
    search-base: ou=people,dc=example,dc=com
    search-filter: "(&(objectClass=person)(uid={username}))"
    email-attribute: mail
    name-attribute: cn
    group-attribute: memberOf            # read groups from user's memberOf attribute
    group-search-base: ""                # if set, active group search instead of memberOf
    group-search-filter: ""              # e.g. (&(objectClass=groupOfNames)(member={dn}))
  auto-assignment:
    - role: admin
      match:
        ldap-groups: ["cn=kafka-admins,ou=groups,dc=example,dc=com"]
    - role: viewer
      match:
        authenticated: true
```

**Group resolution modes** (mutually exclusive):
- **Default (memberOf)**: read `group-attribute` from user entry. Simple, works with AD and OpenLDAP with memberOf overlay.
- **Active search**: when `group-search-base` is set, ignore `group-attribute` and search for groups containing the user DN. For LDAP servers without memberOf.

### Config Extensions

`AutoAssignmentMatch` gains:
```go
LDAPGroups []string `yaml:"ldap-groups"`
```

`validAuthTypes` in `config.go` line 203 updated to include `"ldap"`.

LDAP-specific validation in `Validate()`: when types includes `"ldap"`, require `bind-dn`, `search-base`, `url` non-empty.

### Security Considerations

- **Timing attack mitigation**: When user is not found in LDAP search (step 3), perform a dummy LDAP bind against a known-invalid DN to normalize response time. This mirrors `BasicAuthenticator`'s dummy bcrypt pattern (line 43 of `basic.go`).
- Bind password via `${ENV_VAR}` expansion
- StartTLS recommended; config validation logs warning if `start-tls: false` with non-localhost URL
- Service account should have read-only LDAP access
- Rate limiting: reuses existing `LoginRateLimiter`
- Connection timeout configurable (default 10s) to prevent hanging on unreachable LDAP

### Files

| File | Purpose |
|------|---------|
| `internal/auth/ldap_provider.go` | LDAP authenticator with timing normalization |
| `internal/auth/ldap_provider_test.go` | Tests with mock LDAP server |
| `internal/auth/auto_assignment.go` | Add LDAPGroups match condition |
| `internal/auth/auto_assignment_test.go` | Tests for LDAP group matching |
| `internal/api/handlers/auth.go` | Rename LoginBasic → LoginCredentials, add LDAP dispatch |
| `internal/config/config.go` | LDAPConfig type + validation + validAuthTypes update |
| `internal/config/config_test.go` | Config parsing tests |
| `cmd/kafkaui/main.go` | LDAP init wiring |
| `frontend/src/pages/LoginPage.tsx` | `hasUsernamePassword` check |
| `config.example.yaml` | LDAP config examples |
| `docs/authentication/ldap.md` | Setup guide |

### Dependencies

- `github.com/go-ldap/ldap/v3` — LDAP client

---

## 4. Configuration Wizard

### Purpose

Replace the flat "Add Cluster" form with a guided multi-step wizard. Reduces cognitive load — users fill in only what they need, with test-connection feedback before saving.

### Architecture

Frontend-only feature. The backend already supports everything needed:
- `POST /api/v1/admin/clusters/test` — test connection
- `POST /api/v1/admin/clusters` — save cluster
- `GET /api/v1/admin/clusters` — list clusters (for edit pre-population)

Note: the `AddClusterRequest` type in `api.ts` already includes all fields (TLS, SASL, Schema Registry, Kafka Connect, KSQL, Metrics). The backend `AdminHandler.AddCluster` persists to `dynamic.yaml`. Verify at implementation time that all wizard fields are correctly persisted and reloaded — if any are missing from the dynamic config save/load, fix the backend.

### Wizard Steps

| Step | Fields | Required | Notes |
|------|--------|----------|-------|
| 1. Connection | Name, Bootstrap Servers | Yes | Basic cluster info |
| 2. Security | TLS toggle, CA file path | No | Skip if no TLS |
| 3. Authentication | SASL mechanism, username, password | No | Skip if no auth |
| 4. Schema Registry | URL | No | Skip if not used |
| 5. Kafka Connect | Name + URL (repeatable) | No | Multiple connect clusters |
| 6. KSQL | URL | No | Skip if not used |
| 7. Review & Test | Summary + test connection button | Yes | Visual pass/fail feedback |

### UI Design

- Stepper component at the top showing current step and progress
- "Back" / "Next" / "Skip" buttons per step
- Optional steps show "Skip" alongside "Next"
- Step 7 shows a summary card with all configured values and a "Test Connection" button
- On test success: "Save" button enabled with green checkmark
- On test failure: error message displayed, user can go back and fix

### Component Structure

```
ClusterWizard.tsx (state machine)
├── WizardStepper.tsx (step indicator)
├── ConnectionStep.tsx
├── SecurityStep.tsx
├── AuthStep.tsx
├── SchemaRegistryStep.tsx
├── KafkaConnectStep.tsx
├── KsqlStep.tsx
└── ReviewStep.tsx (summary + test + save)
```

### State Management

Single `useReducer` in `ClusterWizard` holding all form data. Each step component receives relevant slice + dispatch. No TanStack Query needed until the test/save actions.

### Integration

- `SettingsClustersPage.tsx` — "Add Cluster" button opens the wizard (dialog or full-page)
- On save success: invalidate `admin-clusters` query, close wizard
- Edit mode: pre-populate wizard with existing cluster config from `GET /admin/clusters` response

### Files

| File | Purpose |
|------|---------|
| `frontend/src/components/wizard/ClusterWizard.tsx` | Main wizard component + state |
| `frontend/src/components/wizard/WizardStepper.tsx` | Step indicator |
| `frontend/src/components/wizard/steps/*.tsx` | Individual step components |
| `frontend/src/pages/SettingsClustersPage.tsx` | Integration point |

### Dependencies

None — uses existing shadcn components (Dialog, Button, Input, Card, Badge).

---

## Implementation Order

1. **CEL Validation Endpoint** → `feature/cel-validate` → 0.13.1 (small, ~1h)
2. **Custom SerDe** → `feature/custom-serde` → 0.13.2 (largest feature)
3. **LDAP Auth** → `feature/ldap-auth` → 0.13.3
4. **Configuration Wizard** → `feature/config-wizard` → 0.13.4
5. **Tier 2 Review** → fix/ branches → 0.14.0

SerDe is the first major feature because it transforms how messages are displayed — the foundation that CEL, browsing, and live tail all build on. LDAP is independent. Wizard is frontend-only polish, last.

## Cross-cutting Concerns

### Testing Strategy
- Unit tests for all new packages (serde, ldap)
- Integration tests where external services are involved (mock LDAP server, mock Schema Registry)
- Frontend: component tests for wizard steps
- Existing tests must not break

### Config Backwards Compatibility
- All new config sections are optional with sensible defaults
- `auth.types` accepts `"ldap"` as a new valid value (added to `validAuthTypes`)
- `serde` section defaults to auto-detect when absent
- No breaking changes to existing config format

### Error Handling
- CEL: compilation errors returned as 400 with expression position info
- SerDe: deserialization failures fall through to next deserializer, ultimately string fallback
- LDAP: connection failures logged + returned as 401, no info leakage about user existence, timing normalization on user-not-found
- Wizard: test connection errors displayed in-wizard, no silent failures

# Tier 2 Features Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement 4 Tier 2 features: CEL validation endpoint, Custom SerDe, LDAP auth, Configuration Wizard.

**Architecture:** Each feature is an independent unit built in an isolated git worktree on its own branch. Features share no state and can be implemented in any order, though the recommended sequence is CEL → SerDe → LDAP → Wizard. SerDe integrates with the existing CEL filter by replacing raw string values with deserialized output.

**Tech Stack:** Go 1.25, chi/v5, cel-go, goavro/v2, protobuf, go-ldap/v3, React 19, TypeScript, shadcn/ui, TanStack Query.

**Spec:** `docs/superpowers/specs/2026-03-21-tier2-features-design.md`

**Versioning:** 0.13.1 (CEL), 0.13.2 (SerDe), 0.13.3 (LDAP), 0.13.4 (Wizard), 0.14.0 (post-review)

---

## Feature 1: CEL Validation Endpoint (→ 0.13.1)

**Branch:** `feature/cel-validate`
**Scope:** One new handler + test + route wiring. ~30 min.

### Task 1.1: CEL validate handler + test + route

**Files:**
- Create: `internal/api/handlers/cel.go`
- Create: `internal/api/handlers/cel_test.go`
- Modify: `internal/api/router.go` (add route inside auth-protected group, before `/clusters/{clusterName}` block, same level as `/dashboard`)

- [ ] **Step 1: Write the handler**

```go
// internal/api/handlers/cel.go
package handlers

import (
	"net/http"

	celfilter "github.com/Smyrcu/KafkaUI/internal/cel"
)

const maxCELExpressionLen = 1000

// CELHandler handles CEL expression operations.
type CELHandler struct{}

func NewCELHandler() *CELHandler { return &CELHandler{} }

func (h *CELHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Expression string `json:"expression"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Expression == "" {
		writeError(w, http.StatusBadRequest, "expression is required")
		return
	}
	if len(req.Expression) > maxCELExpressionLen {
		writeError(w, http.StatusBadRequest, "expression exceeds maximum length of 1000 characters")
		return
	}
	if _, err := celfilter.NewFilter(req.Expression); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true})
}
```

- [ ] **Step 2: Write handler tests**

```go
// internal/api/handlers/cel_test.go
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCELHandler_Validate(t *testing.T) {
	h := NewCELHandler()
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{"empty expression", `{"expression":""}`, 400, "expression is required"},
		{"too long", `{"expression":"` + strings.Repeat("x", 1001) + `"}`, 400, "maximum length"},
		{"valid expression", `{"expression":"key == \"test\""}`, 200, `"valid":true`},
		{"invalid syntax", `{"expression":"key =="}`, 200, `"valid":false`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/cel/validate", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.Validate(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}
```

- [ ] **Step 3: Wire route in router.go**

In `internal/api/router.go`, inside the auth-protected group, before the `/clusters/{clusterName}` route block (same level as `/dashboard`), add:

```go
celHandler := handlers.NewCELHandler()
```
(at handler construction, around line 48)

```go
r.With(requireAction("view_messages")).Post("/cel/validate", celHandler.Validate)
```
(inside the protected group, around line 105)

- [ ] **Step 4: Run tests**

```bash
go build ./... && go test ./internal/api/...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/cel.go internal/api/handlers/cel_test.go internal/api/router.go
git commit -m "feat(api): add CEL expression validation endpoint"
```

### Task 1.2: Version bump + merge

- [ ] **Step 1: Bump version to 0.13.1** in `frontend/package.json`, `charts/kafkaui/Chart.yaml`, `internal/api/handlers/openapi.yaml`

- [ ] **Step 2: Commit + merge to main**

```bash
git commit -m "chore: bump version to 0.13.1"
git checkout main && git merge feature/cel-validate --no-ff
git tag v0.13.1 && git push origin main --tags
```

---

## Feature 2: Custom SerDe (→ 0.13.2)

**Branch:** `feature/custom-serde`

### SerDe Wiring Architecture

SerDe chains are **per-cluster** (each cluster can have different serde config and schema registry). The wiring approach:

1. In `main.go`, build a `map[string]*serde.Chain` keyed by cluster name
2. Pass this map via `RouterDeps.SerDeChains map[string]*serde.Chain`
3. `MessageHandler` and `LiveTailHandler` receive the map and look up by cluster name at request time
4. Deserialization happens on **raw `[]byte`** from `*kgo.Record` — BEFORE the `string()` conversion in `ConsumeMessages` and `livetail.go`

### Task 2.1: Deserializer interface + string fallback

**Files:**
- Create: `internal/serde/deserializer.go`
- Create: `internal/serde/string.go`
- Create: `internal/serde/string_test.go`
- Create: `internal/serde/deserializer_test.go`

- [ ] **Step 1: Write deserializer interface + Chain**

```go
// internal/serde/deserializer.go
package serde

// Deserializer converts raw Kafka message bytes to a display string.
type Deserializer interface {
	Name() string
	Detect(topic string, data []byte, headers map[string]string) bool
	Deserialize(topic string, data []byte) (string, error)
}

// Chain tries deserializers in order, returning the first successful result.
type Chain struct {
	deserializers []Deserializer
}

func NewChain(ds ...Deserializer) *Chain {
	return &Chain{deserializers: ds}
}

// Deserialize attempts each deserializer in order. On the first successful
// Detect+Deserialize, it returns the result. Falls back to raw string.
func (c *Chain) Deserialize(topic string, data []byte, headers map[string]string) string {
	if len(data) == 0 {
		return ""
	}
	for _, d := range c.deserializers {
		if d.Detect(topic, data, headers) {
			if result, err := d.Deserialize(topic, data); err == nil {
				return result
			}
		}
	}
	return string(data)
}
```

- [ ] **Step 2: Write string deserializer**

```go
// internal/serde/string.go
package serde

import (
	"strings"
	"unicode/utf8"
)

type StringDeserializer struct{}

func (s *StringDeserializer) Name() string                                        { return "string" }
func (s *StringDeserializer) Detect(_ string, _ []byte, _ map[string]string) bool { return true }
func (s *StringDeserializer) Deserialize(_ string, data []byte) (string, error) {
	if utf8.Valid(data) {
		return string(data), nil
	}
	return strings.ToValidUTF8(string(data), "\uFFFD"), nil
}
```

- [ ] **Step 3: Write tests** — string_test.go (valid UTF-8, invalid UTF-8, always detects) + deserializer_test.go (chain priority: first match wins, fallback to string on detect failure)

- [ ] **Step 4: `go test ./internal/serde/...` + commit**

### Task 2.2: JSON deserializer

**Files:** Create `internal/serde/json.go`, `internal/serde/json_test.go`

- [ ] **Step 1: Write JSON deserializer** — `Detect`: `json.Valid(data)`. `Deserialize`: `json.Indent` with 2-space. Tests: pretty-prints compact JSON, non-JSON fails detect.
- [ ] **Step 2: `go test` + commit**

### Task 2.3: Schema Registry GetSchemaByID + cache

**Files:** Modify `internal/schema/client.go`, `internal/schema/client_test.go`

- [ ] **Step 1: Add `schemaCache sync.Map` to Client struct** (zero value ready to use, no init needed in `NewClient`)

- [ ] **Step 2: Add GetSchemaByID method**

```go
func (c *Client) GetSchemaByID(ctx context.Context, id int) (string, error) {
	if cached, ok := c.schemaCache.Load(id); ok {
		return cached.(string), nil
	}
	var resp struct{ Schema string `json:"schema"` }
	if err := c.http.Do(ctx, "GET", fmt.Sprintf("/schemas/ids/%d", id), nil, &resp); err != nil {
		return "", fmt.Errorf("get schema by id %d: %w", id, err)
	}
	c.schemaCache.Store(id, resp.Schema)
	return resp.Schema, nil
}
```

- [ ] **Step 3: Write test with mock HTTP server + commit**

### Task 2.4: Avro deserializer

**Files:** Create `internal/serde/avro.go`, `internal/serde/avro_test.go`

- [ ] **Step 1: `go get github.com/linkedin/goavro/v2`**

- [ ] **Step 2: Write Avro deserializer**

```go
type AvroDeserializer struct {
	schemaClient *schema.Client
	codecs       sync.Map // schema ID -> *goavro.Codec
}

func (a *AvroDeserializer) Detect(_ string, data []byte, _ map[string]string) bool {
	return len(data) >= 5 && data[0] == 0x00
}

func (a *AvroDeserializer) Deserialize(topic string, data []byte) (string, error) {
	schemaID := int(binary.BigEndian.Uint32(data[1:5]))
	codec, err := a.getCodec(schemaID)
	if err != nil {
		return "", err
	}
	native, _, err := codec.NativeFromBinary(data[5:])
	if err != nil {
		return "", fmt.Errorf("avro decode: %w", err)
	}
	jsonBytes, err := json.Marshal(native)
	if err != nil {
		return "", fmt.Errorf("avro to json: %w", err)
	}
	return string(jsonBytes), nil
}

func (a *AvroDeserializer) getCodec(id int) (*goavro.Codec, error) {
	if cached, ok := a.codecs.Load(id); ok {
		return cached.(*goavro.Codec), nil
	}
	schemaJSON, err := a.schemaClient.GetSchemaByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	codec, err := goavro.NewCodec(schemaJSON)
	if err != nil {
		return nil, fmt.Errorf("avro codec for schema %d: %w", id, err)
	}
	a.codecs.Store(id, codec)
	return codec, nil
}
```

- [ ] **Step 3: Write tests** — valid Confluent wire format decodes, too short fails detect, invalid schema ID fails deserialize (falls through in chain)
- [ ] **Step 4: `go test` + commit**

### Task 2.5: Protobuf deserializer

**Files:** Create `internal/serde/protobuf.go`, `internal/serde/protobuf_test.go`

- [ ] **Step 1: Write Protobuf deserializer**

Uses `google.golang.org/protobuf/proto`, `google.golang.org/protobuf/types/descriptorpb`, `google.golang.org/protobuf/reflect/protodesc`, `google.golang.org/protobuf/types/dynamicpb`, `google.golang.org/protobuf/encoding/protojson`.

Key logic:
- `Detect`: `len(data) >= 6 && data[0] == 0x00` + after 4-byte schema ID, next byte is varint-encoded message index count
- `Deserialize`: extract schema ID (bytes 1-4), fetch schema as `.proto` FileDescriptorProto from Schema Registry, parse with `protodesc.NewFile`, get message descriptor by index, create `dynamicpb.NewMessage`, `proto.Unmarshal`, `protojson.Marshal`
- Schema + descriptor cache: `sync.Map` keyed by schema ID

- [ ] **Step 2: Write tests + commit**

### Task 2.6: SerDe config types

**Files:** Modify `internal/config/config.go`, `internal/config/config_test.go`

- [ ] **Step 1: Add types**

```go
type SerDeConfig struct {
	Default string      `yaml:"default"` // "auto" (default), "json", "string", "avro", "protobuf"
	Rules   []SerDeRule `yaml:"rules"`
}

type SerDeRule struct {
	TopicPattern      string `yaml:"topic-pattern"`
	KeyDeserializer   string `yaml:"key-deserializer"`
	ValueDeserializer string `yaml:"value-deserializer"`
}
```

Add `SerDe SerDeConfig` (yaml tag `"serde"`) to `ClusterConfig`. Topic patterns use `regexp.MatchString` — same as `DataMaskingConfig`.

- [ ] **Step 2: Tests + commit**

### Task 2.7a: Chain builder (registry.go)

**Files:** Create `internal/serde/registry.go`

- [ ] **Step 1: Write BuildChain function**

```go
func BuildChain(cfg config.SerDeConfig, schemaClient *schema.Client) *Chain {
	// Build ordered deserializer list based on config.Default
	// "auto" (default): Avro → Protobuf → JSON → String
	// explicit: only the named deserializer + String fallback
}
```

- [ ] **Step 2: Test + commit**

### Task 2.7b: Integration into message browsing + live tail

**Files:**
- Modify: `internal/kafka/client.go` (use raw `[]byte` from `*kgo.Record` before `string()` conversion)
- Modify: `internal/api/ws/livetail.go`
- Modify: `internal/api/handlers/message.go`

- [ ] **Step 1: Add SerDe chain to MessageHandler and LiveTailHandler**

`MessageHandler` gains `serdeChains map[string]*serde.Chain`. In `Browse`, look up chain by cluster name, call `chain.Deserialize(topic, r.Key, headers)` and `chain.Deserialize(topic, r.Value, headers)` on the raw `*kgo.Record` bytes BEFORE constructing `MessageRecord`.

Same pattern for `LiveTailHandler` — receives `serdeChains`, looks up by cluster name from URL param.

Important: deserialization happens on raw `r.Key` and `r.Value` (`[]byte` from `*kgo.Record`), NOT on `MessageRecord.Key`/`MessageRecord.Value` which are already `string()` converted.

- [ ] **Step 2: Run all tests + commit**

### Task 2.7c: Wiring in main.go + deps

**Files:**
- Modify: `internal/api/deps.go` — add `SerDeChains map[string]*serde.Chain`
- Modify: `cmd/kafkaui/main.go` — build per-cluster chains
- Modify: `config.example.yaml` — add serde examples

- [ ] **Step 1: Build per-cluster chains in main.go**

```go
serdeChains := make(map[string]*serde.Chain)
for _, cc := range cfg.Clusters {
	var schemaClient *schema.Client
	if cc.SchemaRegistry.URL != "" {
		schemaClient = schema.NewClient(cc.SchemaRegistry.URL)
	}
	serdeChains[cc.Name] = serde.BuildChain(cc.SerDe, schemaClient)
}
```

Pass `serdeChains` into `RouterDeps`, then into `MessageHandler` and `LiveTailHandler` constructors.

- [ ] **Step 2: Run all tests + commit**

### Task 2.8: Version bump + merge (→ 0.13.2)

---

## Feature 3: LDAP Authentication (→ 0.13.3)

**Branch:** `feature/ldap-auth`

### Task 3.1: LDAP config types + validation

**Files:** Modify `internal/config/config.go`, `internal/config/config_test.go`

- [ ] **Step 1: Add LDAPConfig type**

```go
type LDAPConfig struct {
	URL               string `yaml:"url"`
	StartTLS          bool   `yaml:"start-tls"`
	ConnectionTimeout string `yaml:"connection-timeout"` // parsed via time.ParseDuration, default "10s"
	BindDN            string `yaml:"bind-dn"`
	BindPassword      string `yaml:"bind-password"`
	SearchBase        string `yaml:"search-base"`
	SearchFilter      string `yaml:"search-filter"`      // default: (&(objectClass=person)(uid={username}))
	EmailAttribute    string `yaml:"email-attribute"`     // default: mail
	NameAttribute     string `yaml:"name-attribute"`      // default: cn
	GroupAttribute    string `yaml:"group-attribute"`      // default: memberOf
	GroupSearchBase   string `yaml:"group-search-base"`    // if set, active search instead of memberOf
	GroupSearchFilter string `yaml:"group-search-filter"`  // e.g. (&(objectClass=groupOfNames)(member={dn}))
}

// ConnectionTimeoutDuration parses ConnectionTimeout as time.Duration.
// Returns 10s default if empty or invalid.
func (c LDAPConfig) ConnectionTimeoutDuration() time.Duration {
	if c.ConnectionTimeout == "" {
		return 10 * time.Second
	}
	d, err := time.ParseDuration(c.ConnectionTimeout)
	if err != nil {
		return 10 * time.Second
	}
	return d
}
```

Add `LDAP LDAPConfig` (yaml `"ldap"`) to `AuthConfig`.
Add `"ldap"` to `validAuthTypes` slice.
Add `LDAPGroups []string` (yaml `"ldap-groups"`) to `AutoAssignmentMatch`.

Validation: when types includes `"ldap"`, require `url`, `bind-dn`, `search-base` non-empty. Parse `ConnectionTimeout` at validation time to catch invalid durations early. Warn if `start-tls: false` with non-localhost URL.

- [ ] **Step 2: Tests + commit**

### Task 3.2: LDAP authenticator

**Files:** Create `internal/auth/ldap_provider.go`, `internal/auth/ldap_provider_test.go`

- [ ] **Step 1: `go get github.com/go-ldap/ldap/v3`**

- [ ] **Step 2: Write LDAPAuthenticator**

```go
type LDAPAuthenticator struct {
	cfg    config.LDAPConfig
	logger *slog.Logger
}

func NewLDAPAuthenticator(cfg config.LDAPConfig, logger *slog.Logger) *LDAPAuthenticator {
	return &LDAPAuthenticator{cfg: cfg, logger: logger}
}

func (a *LDAPAuthenticator) Authenticate(username, password string) (*UserIdentity, error) {
	// 1. Dial with timeout
	// 2. StartTLS if configured
	// 3. Service bind
	// 4. Search for user
	// 5. If NOT found: dummy bind against invalid DN for timing normalization, return generic error
	// 6. User bind with found DN + password
	// 7. Extract email (mail attr), name (cn attr), groups (memberOf or group search)
	// 8. Return UserIdentity{ProviderName: "ldap", ProviderType: "ldap", ExternalID: userDN, ...}
}
```

Timing normalization: on user-not-found, perform `conn.Bind("cn=dummy-timing-normalization,dc=invalid", password)` — this will fail but takes similar time to a real bind, preventing user enumeration.

- [ ] **Step 3: Write tests**

Use `github.com/jimlambrt/gldap` for an in-process mock LDAP server. Test cases:
- Valid credentials → UserIdentity with email, name, groups
- Invalid password → error
- User not found → error (timing normalized)
- Connection timeout → error

- [ ] **Step 4: `go test ./internal/auth/...` + commit**

### Task 3.3: Auto-assignment LDAPGroups match

**Files:** Modify `internal/auth/auto_assignment.go`, `internal/auth/auto_assignment_test.go`

- [ ] **Step 1: Add LDAPGroups condition** in `matchesRule()`, after GitLabGroups block:

```go
if len(match.LDAPGroups) > 0 {
	conditions++
	// LDAP groups stored in identity.Orgs. Case-insensitive via hasOverlap.
	// Note: LDAP DNs are case-insensitive per RFC 4514 for attribute types;
	// hasOverlap lowercases both sides which works for most AD/OpenLDAP setups.
	if hasOverlap(identity.Orgs, match.LDAPGroups) {
		matched++
	}
}
```

- [ ] **Step 2: Tests + commit**

### Task 3.4: Handler + frontend + wiring

**Files:**
- Modify: `internal/api/handlers/auth.go`
- Modify: `internal/api/deps.go`
- Modify: `cmd/kafkaui/main.go`
- Modify: `frontend/src/pages/LoginPage.tsx`

- [ ] **Step 1: Add LDAP to AuthHandler**

Add `ldap *auth.LDAPAuthenticator` to `AuthHandler` and `AuthHandlerDeps`.

- [ ] **Step 2: Update LoginBasic for LDAP dispatch**

Pre-declare variables to avoid zero-value bug:
```go
var identity *auth.UserIdentity
var authErr error

if h.hasType("ldap") && h.ldap != nil {
	identity, authErr = h.ldap.Authenticate(req.Username, req.Password)
}
if authErr != nil && h.hasType("basic") && h.basic != nil {
	identity, authErr = h.basic.Authenticate(req.Username, req.Password)
} else if identity == nil && h.hasType("basic") && h.basic != nil {
	identity, authErr = h.basic.Authenticate(req.Username, req.Password)
}
if authErr != nil || identity == nil {
	h.logger.Warn("login failed", "username", req.Username, "ip", ip)
	writeError(w, http.StatusUnauthorized, "invalid credentials")
	return
}
```

- [ ] **Step 3: Wire in main.go**

- [ ] **Step 4: Update LoginPage.tsx** — `hasBasic` → `hasUsernamePassword` = includes "basic" OR "ldap"

- [ ] **Step 5: Add `docs/authentication/ldap.md` + config.example.yaml entries**

- [ ] **Step 6: Run all tests + commit**

### Task 3.5: Version bump + merge (→ 0.13.3)

---

## Feature 4: Configuration Wizard (→ 0.13.4)

**Branch:** `feature/config-wizard`

### Task 4.1: WizardStepper component

**Files:** Create `frontend/src/components/wizard/WizardStepper.tsx`

- [ ] **Step 1: Write stepper** — horizontal step indicator with active/completed/pending states. Props: `steps: string[]`, `currentStep: number`, `onStepClick`.
- [ ] **Step 2: Commit**

### Task 4.2a: Core step components

**Files:** Create ConnectionStep.tsx, SecurityStep.tsx, AuthStep.tsx

- [ ] **Step 1: Write steps** — each receives `data` slice + `onChange`. ConnectionStep: name + bootstrapServers (required). SecurityStep: TLS toggle + CA file. AuthStep: SASL mechanism select + username + password.
- [ ] **Step 2: `tsc -b` + commit**

### Task 4.2b: Optional step components

**Files:** Create SchemaRegistryStep.tsx, KafkaConnectStep.tsx, KsqlStep.tsx

- [ ] **Step 1: Write steps** — SchemaRegistry: URL input. KafkaConnect: repeatable name+URL pairs. Ksql: URL input.
- [ ] **Step 2: `tsc -b` + commit**

### Task 4.2c: Review step

**Files:** Create ReviewStep.tsx

- [ ] **Step 1: Write ReviewStep** — summary card with all configured values. "Test Connection" button with loading/success/error states. "Save" button enabled after test success (or skip).
- [ ] **Step 2: `tsc -b` + commit**

### Task 4.3: ClusterWizard main component

**Files:** Create `frontend/src/components/wizard/ClusterWizard.tsx`

- [ ] **Step 1: Write wizard** — `useReducer` state machine. Renders WizardStepper + current step + navigation buttons. Test via `api.admin.testConnection`, save via `api.admin.addCluster`. Edit mode: accepts `initialData` prop.
- [ ] **Step 2: `tsc -b` + commit**

### Task 4.4: Integration into SettingsClustersPage

**Files:** Modify `frontend/src/pages/SettingsClustersPage.tsx`

- [ ] **Step 1: Replace inline dialog with ClusterWizard**
- [ ] **Step 2: `tsc -b && vitest run` + commit**

### Task 4.5: Version bump + merge (→ 0.13.4)

---

## Post-Implementation

After all 4 features are merged:
1. Full code review (5 parallel agents, same pattern as auth review)
2. Fix issues on fix/ branches
3. Bump to 0.14.0

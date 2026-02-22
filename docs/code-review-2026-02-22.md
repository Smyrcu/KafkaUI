# KafkaUI — Pełna Analiza Kodu (22.02.2026)

> Analiza przeprowadzona przez 21 równoległych agentów AI, pokrywająca cały codebase.

---

## Spis treści

1. [Podsumowanie projektu](#1-podsumowanie-projektu)
2. [Krytyczne problemy](#2-krytyczne-problemy)
3. [Ważne problemy — Backend](#3-ważne-problemy--backend)
4. [Ważne problemy — Frontend](#4-ważne-problemy--frontend)
5. [Analiza architektury](#5-analiza-architektury)
6. [Analiza bezpieczeństwa](#6-analiza-bezpieczeństwa)
7. [Pokrycie testami](#7-pokrycie-testami)
8. [Analiza per-moduł](#8-analiza-per-moduł)
   - [8.1 Entry point & main](#81-entry-point--main)
   - [8.2 Kafka client](#82-kafka-client)
   - [8.3 API router & middleware](#83-api-router--middleware)
   - [8.4 Broker & cluster handlers](#84-broker--cluster-handlers)
   - [8.5 Topic handlers](#85-topic-handlers)
   - [8.6 Message handlers](#86-message-handlers)
   - [8.7 Consumer group handlers](#87-consumer-group-handlers)
   - [8.8 Schema registry](#88-schema-registry)
   - [8.9 Kafka Connect](#89-kafka-connect)
   - [8.10 KSQL & ACL](#810-ksql--acl)
   - [8.11 Auth & dashboard](#811-auth--dashboard)
   - [8.12 WebSocket live tail](#812-websocket-live-tail)
   - [8.13 Config system](#813-config-system)
   - [8.14 Frontend — app structure](#814-frontend--app-structure)
   - [8.15 Frontend — API client](#815-frontend--api-client)
   - [8.16 Frontend — core pages](#816-frontend--core-pages)
   - [8.17 Frontend — secondary pages](#817-frontend--secondary-pages)
   - [8.18 Frontend — test infrastructure](#818-frontend--test-infrastructure)
   - [8.19 OpenAPI spec](#819-openapi-spec)
   - [8.20 Docker & build system](#820-docker--build-system)
9. [Stan projektu vs plany iteracji](#9-stan-projektu-vs-plany-iteracji)
10. [Top 10 rekomendacji](#10-top-10-rekomendacji)

---

## 1. Podsumowanie projektu

| Aspekt | Wartość |
|--------|--------|
| **Stack** | Go 1.25 backend + React 19 frontend, single binary via `go:embed` |
| **Backend** | chi/v5 router, franz-go Kafka client, slog logging |
| **Frontend** | Vite 7, TypeScript 5.9, shadcn/ui, Tailwind CSS v4, TanStack Query v5 |
| **WebSocket** | gorilla/websocket (live tail) |
| **Rozmiar** | ~40 plików Go, ~35 plików React, ~4500 LOC |
| **Stan** | Wersja wczesna/prototypowa z solidnymi fundamentami ale istotnymi lukami |

---

## 2. Krytyczne problemy

### 2.1 Auth middleware NIE jest podpięty

Middleware `Auth` i `RequireAction` istnieją w kodzie (`internal/api/middleware/auth.go`), ale **nigdy nie są aplikowane na żadne route** w `router.go`. Wszystkie endpointy API — w tym tworzenie/usuwanie topicow, zarządzanie ACL, wykonywanie KSQL, produkcja wiadomości — są dostępne bez uwierzytelnienia, niezależnie od konfiguracji `auth.enabled`.

**Pliki:** `internal/api/router.go`, `internal/api/middleware/auth.go`

### 2.2 Hardcoded default session secret

```go
sessionSecret = "kafkaui-default-secret-change-me"
```

Ktokolwiek znający ten string może podrabiać cookies sesyjne. Brak ostrzeżenia w logach gdy default jest używany.

**Plik:** `cmd/kafkaui/main.go:43-46`

### 2.3 Trasy auth niezarejestrowane

`Login`, `Callback`, `Logout`, `Me` — handlery istnieją w `AuthHandler`, ale tylko `/auth/status` jest zarejestrowany w routerze. Flow OIDC jest niefunkcjonalny.

**Pliki:** `internal/api/handlers/auth.go`, `internal/api/router.go:48`

### 2.4 CORS wildcard + credentials

```go
AllowedOrigins: []string{"*"}, AllowCredentials: true
```

Narusza specyfikację CORS. Efektywnie pozwala dowolnej stronie na wykonanie uwierzytelnionych requestów.

**Plik:** `internal/api/router.go:24-30`

### 2.5 Brak SPA fallback

Bezpośrednia nawigacja do client-side routes (np. `/clusters/local/topics`) zwraca 404 z Go file servera. Brak fallbacku do `index.html` dla client-side routingu React.

**Plik:** `cmd/kafkaui/main.go:58-61`

---

## 3. Ważne problemy — Backend

| # | Problem | Lokalizacja | Wpływ |
|---|---------|------------|-------|
| 1 | `os.Exit(1)` w goroutine omija `defer registry.Close()` | `main.go:72-78` | Resource leak przy startup failure |
| 2 | `WriteTimeout: 0` globalnie (nie tylko WS) | `main.go:68` | Slow-write DoS na REST endpointach |
| 3 | Brak `http.MaxBytesReader` na żadnym endpoincie | Wszystkie handlery | Memory exhaustion via duże requesty |
| 4 | Nowy `schema.Client`/`connect.Client`/`ksql.Client` na każdy request | `schema.go:32`, `connect.go:23`, `ksql.go:32` | Brak connection reuse, port exhaustion |
| 5 | Nowy Kafka consumer na każde browse/live tail | `client.go:302`, `livetail.go:63` | Kosztowne TCP connections per request |
| 6 | ACL `stringTo*` defaultuje do `Any` przy typo | `acl.go` | Niechciany ACL z szerokim scope |
| 7 | `Offset = -1` (latest) czeka na nowe msg zamiast ostatnich N | `client.go:326` | 10s timeout na pustych topikach |
| 8 | Config `BasePath` parsowany ale nigdzie nie używany | `config.go` + `main.go` | Martwa feature |
| 9 | Data masking engine istnieje ale nie jest podpięty | `masking/engine.go` | Brak masking mimo konfiguracji |
| 10 | RBAC rules parsowane ale nigdy nie instantiated | `config.go` + `router.go` | Brak authorization |
| 11 | `TopicDetails` cicho ignoruje błąd describe config | `client.go:232` | Puste configs bez ostrzeżenia |
| 12 | Schema/Connect N+1 HTTP problem | `schema/client.go:57-91` | 1 req/subject; 101 req na 100 schemas |
| 13 | `err.Error()` wyciek do klienta HTTP | Wszystkie handlery | Ekspozycja adresów brokerów, auth details |
| 14 | Brak duplikat cluster name detection | `registry.go:26-35` | Cichy overwrite |
| 15 | WebSocket `CheckOrigin` zawsze true | `livetail.go:22` | Cross-site WS hijacking |
| 16 | Brak write deadline na WebSocket | `livetail.go:120,146` | Slow client = zablokowany goroutine |
| 17 | Brak binarnych danych handling (string() na non-UTF8) | `client.go:393-394`, `livetail.go:139-140` | Garbled output dla Avro/Protobuf |
| 18 | Session `ClearSession` używa niezainicjalizowanego `sm.secure` | `session.go:100` | Cookie clear fail na HTTPS |
| 19 | Brak topic config update endpoint | `topic.go` | Brak edycji retention, cleanup policy |
| 20 | Connector Create zawsze na pierwszym Connect cluster | `connect.go:109` | Brak wyboru target cluster |

---

## 4. Ważne problemy — Frontend

| # | Problem | Lokalizacja | Wpływ |
|---|---------|------------|-------|
| 1 | Brak 404 catch-all route | `App.tsx` | Pusta strona na nieznanych URL |
| 2 | Brak React Error Boundary | `App.tsx` | Runtime error = crash całej apki |
| 3 | `ConnectorDetailPage` — brak confirm przy delete | `ConnectorDetailPage.tsx:139` | Przypadkowe usunięcie connectora |
| 4 | WebSocket nie robi reconnect | `useWebSocket.ts` | Disconnect = cisza |
| 5 | `JSON.parse` bez try/catch w WS hook | `useWebSocket.ts:25` | Crash na malformed message |
| 6 | `any` type w KSQL response | `api.ts:159` | Brak type safety |
| 7 | Brak URL encoding dla cluster/topic names | `api.ts` | Zepsute URL ze special chars |
| 8 | Header merge bug (latentny) — spread nadpisze Content-Type | `api.ts:5` | Zepsute custom headers |
| 9 | Native `<select>` zamiast shadcn Select | `TopicMessagesPage`, `ConsumerGroupDetailPage` | UI niespójność |
| 10 | Brak paginacji w żadnym widoku listy | Wszystkie list pages | Problemy wydajnościowe na dużych klastrach |
| 11 | `clusterName!` non-null assertions w mutations | Wiele stron | Potencjalny undefined w API call |
| 12 | Duplikaty badge variant helpers w każdej stronie | 5+ stron | Code duplication |
| 13 | `ACLEntry` type zduplikowany (api.ts + AclPage.tsx) | `AclPage.tsx:14` | Rozsynchronizowanie typów |
| 14 | Brak `staleTime` — każdy mount triggeruje refetch | `App.tsx` QueryClient | Zbędne requesty |
| 15 | `PlaceholderPage` component nieużywany (dead code) | `PlaceholderPage.tsx` | Zbędny plik |

---

## 5. Analiza architektury

### Mocne strony

| Aspekt | Ocena | Komentarz |
|--------|-------|-----------|
| Dependency injection | Doskonały | Registry → Handler, brak globals/init() |
| Context propagation | Doskonały | `r.Context()` wszędzie, timeout na browse (10s) |
| Error wrapping | Dobry | Konsystentne `fmt.Errorf("ctx: %w", err)` |
| Frontend state management | Dobry | TanStack Query + lokalne state, brak over-engineering |
| Shadcn/ui design system | Dobry | 15 komponentów, spójne theming (light/dark/system) |
| Single binary deploy | Doskonały | `go:embed` — eleganckie, zero external dependencies |
| Multi-cluster support | Dobry | Registry pattern z zachowaniem kolejności |
| SASL/TLS support | Dobry | PLAIN, SCRAM-SHA-256/512 + TLS 1.2+ |
| Docker multi-stage build | Dobry | 3-stage, layer caching, Alpine final (~20-40MB) |
| Handler pattern consistency | Dobry | `writeJSON`/`writeError` helpers, uniform error format |
| Frontend routing | Dobry | React Router v7, spójne URL: `/clusters/:name/...` |

### Słabe strony

| Aspekt | Ocena | Komentarz |
|--------|-------|-----------|
| Auth/RBAC wiring | Krytyczny | Kod istnieje ale nie działa |
| Interface-based design | Słaby | `kafka.Client` to struct — brak mockowania |
| Connection reuse | Słaby | Nowy HTTP/Kafka client per request |
| Pagination | Brak | Żaden endpoint nie wspiera paginacji |
| Config validation | Słaby | Tylko port default, brak walidacji wymaganych pól |
| CI/CD | Brak | Żaden pipeline nie istnieje |
| Error granularity | Słaby | Wszystko to 500, brak 404/409/403 mapping |
| Binary data handling | Brak | `string()` na non-UTF8 = garbled output |

---

## 6. Analiza bezpieczeństwa

### Krytyczne

| # | Problem | Opis |
|---|---------|------|
| 1 | Auth nie działa | Middleware istnieje ale nie jest podpięty — all endpoints public |
| 2 | Default session secret | Hardcoded `"kafkaui-default-secret-change-me"` — forgowalny |
| 3 | CORS misconfiguration | `*` + credentials narusza spec CORS |
| 4 | WebSocket origin bypass | `CheckOrigin: true` — cross-site WS hijacking |
| 5 | KSQL query forwarding | Dowolne KSQL (DROP, CREATE, INSERT) bez ograniczeń |

### Ważne

| # | Problem | Opis |
|---|---------|------|
| 6 | Brak request body size limit | Brak `http.MaxBytesReader` — memory exhaustion DoS |
| 7 | Internal error leakage | `err.Error()` zwracany klientowi — broker addresses, auth details |
| 8 | Raw ID token w cookie | OIDC token w base64 (nie encrypted) w session cookie |
| 9 | Open redirect w callback | `redirect_uri` cookie bez walidacji |
| 10 | ACL silent wildcard | Typo w operation → `Any` scope zamiast error |
| 11 | SASL passwords w plaintext config | Brak wymuszenia `${ENV_VAR}` dla secrets |
| 12 | `WriteTimeout: 0` globalnie | Slow-write attack na REST endpoints |
| 13 | No rate limiting | Produce, KSQL execute, ACL create bez limitów |

---

## 7. Pokrycie testami

| Obszar | # Testów | Happy Path? | Ocena |
|--------|----------|:-----------:|-------|
| Kafka client (`client.go`) | 5 | Nie | **Niski** — tylko construction |
| Kafka registry | 6 | Tak | Adekwatny |
| API handlery (wszystkie ~50) | ~50 | Nie — tylko 404/400 | **Niski** |
| Schema client | 7 | Tak | Dobry |
| Connect client | 10 | Tak (częściowo) | Dobry |
| KSQL client | 7 | Tak | Dobry |
| ACL (kafka) | 0 | — | **Brak** |
| Router | 4 | Nie | Niski |
| Auth middleware | 0 | — | **Brak** |
| Session manager | 5 | Tak | Dobry |
| RBAC | 9 | Tak | Dobry |
| Masking engine | 9 | Tak | Dobry |
| Config loading | 12 | Tak | Dobry |
| Frontend API client | 12 | Tak (częściowo) | Umiarkowany |
| Frontend pages | 8 (2 pliki) | Tak | **Niski** — 2/17 stron |
| WebSocket hook | 0 | — | **Brak** |
| Logging middleware | 6 | Tak | Dobry |

**Główna przeszkoda testowania**: `kafka.Client` jest struct, nie interface — mockowanie niemożliwe. Handler testy mogą testować tylko walidację i cluster-not-found paths.

**Frontend**: Tylko `ClustersPage` i `ConsumerGroupsPage` mają testy. 15 stron i oba hooki (`useWebSocket`, `use-mobile`) nietestowane. Brak testów dla API methods: `schemas.*`, `connect.*`, `ksql.*`, `acl.*`, `dashboard.*`.

---

## 8. Analiza per-moduł

### 8.1 Entry point & main

**Plik:** `cmd/kafkaui/main.go`

**Startup sequence:** Flag parsing → Logger init (slog JSON) → Config loading → Kafka registry → Session manager → Router → Frontend file server → HTTP mux → Async listen → Signal wait + graceful shutdown (10s).

**Problemy:**
- `os.Exit(1)` w goroutine omija deferred cleanup (`registry.Close()`)
- Default session secret hardcoded
- `WriteTimeout: 0` globalnie
- Logger level hardcoded na INFO (brak konfiguracji)
- Brak health check endpoint
- Brak SPA fallback
- Brak version/build info injection

**Pozytywne:** Clean DI, proper signal handling (SIGINT/SIGTERM), `go:embed` for single binary, graceful shutdown with bounded context.

### 8.2 Kafka client

**Pliki:** `internal/kafka/client.go` (615 lines), `auth.go`, `acl.go`, `registry.go`

**Operacje:** Brokers, Topics CRUD, TopicDetails, ConsumeMessages, ProduceMessage, ConsumerGroups, ConsumerGroupDetails, ResetOffsets, ACL CRUD.

**Problemy:**
- Brak interface — uniemożliwia mock testing
- Nowy consumer per browse/live tail (kosztowne)
- `Offset = -1` (latest) czeka na nowe msg zamiast browse ostatnich N
- `TopicDetails` cicho ignoruje config describe error
- ACL string converters defaultują do `Any` na unknown input
- SASL/TLS setup zduplikowany w `livetail.go`
- Brak mTLS support (client certificates)
- Brak duplikat cluster name detection w Registry
- Brak `OAUTHBEARER` SASL support

**Pozytywne:** Clean franz-go usage, proper context propagation, consistent error wrapping, defensive nil checks, thread-safe design.

### 8.3 API router & middleware

**Pliki:** `internal/api/router.go`, `internal/api/middleware/`

**Route structure:** `/api/v1/` z chi nesting, `/ws/` osobno. 10 handler structs + auth handler + swagger.

**Middleware chain:** Recoverer → RequestID → Logger → CORS

**Problemy:**
- Auth middleware NIE podpięty (krytyczne)
- CORS `*` + credentials (krytyczne)
- Cluster lookup pattern (3 linie) zduplikowany ~20x w handlerach
- Auth middleware zwraca `text/plain` zamiast JSON
- Logger nie loguje request ID
- Logger zawsze na INFO (brak WARN/ERROR dla 4xx/5xx)
- `responseWriter` nie implementuje `http.Flusher`/`http.Hijacker`
- RequireAction passes when user is nil (silent bypass)

**Pozytywne:** Clean route organization, handler DI via constructor, reusable error responses.

### 8.4 Broker & cluster handlers

**Pliki:** `internal/api/handlers/cluster.go`, `broker.go`

**Shared helpers:** `writeJSON(w, status, data)` i `writeError(w, status, msg)`.

**Pattern:** Każdy handler: lookup cluster → validate input → call kafka client → write response.

**Problemy:**
- `writeJSON` ignoruje error z `json.Encode`
- Brak request body size limit
- Brak Content-Type check na POST/PUT
- Brak interface-based testing
- Inconsistent delete responses (some include resource name, some don't)

### 8.5 Topic handlers

**Plik:** `internal/api/handlers/topic.go` (106 lines)

**Operations:** List, Details, Create, Delete.

**Problemy:**
- Brak topic config update endpoint (PUT)
- Brak `Configs` field w `CreateTopicRequest` — topics tworzone tylko z defaults
- Tylko empty name validation — brak regex, max length, reserved names
- Wszystkie Kafka errors → 500 (brak 404 topic not found, 409 already exists)
- Replica count z pierwszej partycji only — mylące po reassignment

**Testy:** 6 testów — tylko error paths, brak happy-path.

### 8.6 Message handlers

**Plik:** `internal/api/handlers/message.go` (117 lines)

**Features:** Browse (partition, offset/keyword/timestamp, limit 1-500), Produce (key, value, headers, partition).

**Problemy:**
- Brak server-side content filtering (key/value regex)
- Brak cursor-based pagination (no `nextOffset`/`hasMore`)
- 10s timeout = pusty topic czeka pełne 10s
- Brak deserialization (binary = garbled `string()` output)
- Brak integration z Schema Registry
- Produce returns 200 zamiast 201
- Nowy Kafka consumer per request (kosztowne)

**Testy:** 9 testów — dobra walidacja input, brak happy-path.

### 8.7 Consumer group handlers

**Plik:** `internal/api/handlers/consumer_group.go`

**Operations:** List (z state, member count, coordinator), Details (members, per-partition lag), Reset offsets (earliest/latest only).

**Lag calculation:** Correct: `endOffset - currentOffset`, clamped to 0.

**Problemy:**
- Brak Delete consumer group endpoint
- Reset tylko earliest/latest (brak specific offset/timestamp)
- Brak per-partition reset granularity
- Brak sprawdzenia group state przed reset (musi być Empty)
- Brak total lag across topics w response

**Testy:** 6 testów — validation paths only.

### 8.8 Schema registry

**Pliki:** `internal/api/handlers/schema.go`, `internal/schema/client.go`

**Operations:** List subjects, Details (versions), Create, Delete.

**Problemy:**
- **N+1 HTTP**: ListSubjects robi 1 req per subject dla latest version
- **N+1 HTTP**: GetSubjectDetails robi 1 req per version
- Nowy `schema.Client` (+ `http.Client`) per API request
- Brak Schema Registry authentication support
- Brak compatibility testing endpoint
- Brak compatibility level management (set/update)
- Brak schema references (Protobuf/JSON)
- `strings.NewReader(string(encoded))` — unnecessary allocation
- Custom `contains` helper in tests zamiast `strings.Contains`

**Testy:** Handler: 8 (error paths only), Client: 7 (good, httptest-based).

### 8.9 Kafka Connect

**Pliki:** `internal/api/handlers/connect.go`, `internal/connect/client.go`

**Operations:** List, Details, Create, Update, Delete, Restart, Pause, Resume.

**Problemy:**
- Nowy `connect.Client` per request (brak connection reuse)
- Create zawsze na `clients[0]` (brak wyboru target cluster)
- `findConnector` cicho swallows errors (network error = "not found")
- Brak task-level restart (`/tasks/{taskId}/restart`)
- Brak connector plugins listing
- Brak connector validation endpoint
- List results not sorted (map iteration = random order)

**Testy:** Handler: 12 (error paths), Client: 10 (good, brak UpdateConnector test).

### 8.10 KSQL & ACL

**Pliki:** `internal/api/handlers/ksql.go`, `acl.go`, `internal/ksql/client.go`, `internal/kafka/acl.go`

**KSQL:** Execute arbitrary statements, Get server info. Client uses 30s timeout, ksqlDB-specific headers.

**ACL:** List all (wildcard describe), Create single, Delete by filter. Uses `kmsg` raw protocol.

**Problemy:**
- KSQL: dowolne zapytania (DROP, CREATE, INSERT) bez ograniczeń
- KSQL: brak streaming queries support
- KSQL: brak auth dla ksqlDB server
- KSQL: tylko pierwszy element response array przetwarzany
- ACL: `stringToResourceType` defaultuje do `Any` na typo
- ACL: Delete walidacja luźniejsza niż Create (2 vs 6 pól)
- ACL: List bez filtrowania (zawsze all ACLs)
- ACL client: zero testów

**Testy:** KSQL handler: 6 (error), KSQL client: 7 (good). ACL handler: 9 (mixed), ACL client: 0.

### 8.11 Auth & dashboard

**Pliki:** `internal/api/handlers/auth.go`, `dashboard.go`, `internal/auth/`, `internal/masking/`

**Auth:** OIDC Authorization Code flow, HMAC-SHA256 session cookies, RBAC z 14 actions.

**Dashboard:** Parallel cluster health check (brokers → topics → consumer groups), status: healthy/degraded/unreachable.

**Problemy:**
- Auth routes (login/callback/logout/me) niezarejestrowane
- Auth middleware niepodpięty
- RBAC middleware niepodpięty
- Data masking engine nigdzie nie instantiated
- `ClearSession` używa niezainicjalizowanego `sm.secure` field
- `AuthConfig.Type` field never checked
- Brak token refresh/re-verification
- Open redirect via `redirect_uri` cookie
- Brak basic auth support (tylko OIDC)
- Dashboard: brak throughput metrics, partition info, historical data

**Testy:** Session: 5, RBAC: 9, Masking: 9. Auth middleware: 0, Dashboard: 0, Auth handler: 0.

### 8.12 WebSocket live tail

**Plik:** `internal/api/ws/livetail.go` (152 lines)

**Architecture:** Upgrade → Create dedicated consumer (AtEnd, all partitions) → Read loop goroutine + Write/consume loop → Cleanup via defers.

**Problemy:**
- `CheckOrigin` zawsze true (cross-site hijacking)
- Brak write deadline → slow client blocks goroutine
- Brak server-side filtering (partition, key, value)
- Brak backpressure handling (no buffering, no dropping)
- Brak connection limits/tracking
- Brak graceful WebSocket close message
- Binary data corruption (`string()` na non-UTF8)
- SASL/TLS setup zduplikowany z `client.go`
- Brak topic existence validation

**Testy:** 7 — good pre-upgrade error coverage, no message streaming tests.

### 8.13 Config system

**Plik:** `internal/config/config.go`

**Structure:** YAML z `${ENV_VAR}` expansion, multi-cluster, TLS/SASL, OIDC, RBAC, data masking.

**Problemy:**
- Prawie zero walidacji (tylko port default)
- Brak required field checks (cluster name, bootstrap servers)
- Brak duplikat cluster name detection
- `${NONEXISTENT}` = silent empty string
- Brak `${VAR:-default}` syntax
- Brak config hot-reload
- Brak mTLS support (client certs)
- Brak `OAUTHBEARER` SASL
- `BasePath` parsed but ignored

**Testy:** 12 — good coverage of parsing, gap on masking/OIDC/RBAC config parsing.

### 8.14 Frontend — app structure

**Pliki:** `frontend/src/App.tsx`, `main.tsx`, `vite.config.ts`, `package.json`

**Architecture:** React 19 + React Router v7 + TanStack Query v5 + shadcn/ui (15 components) + Tailwind CSS v4.

**Routing:** 14 routes, all nested under `<Layout />` wrapper. URL pattern: `/clusters/:clusterName/<resource>`.

**Problemy:**
- Brak 404 catch-all route
- Brak React Error Boundary
- Brak lazy loading (`React.lazy`)
- `PlaceholderPage` component unused (dead code)
- Sidebar nie responsive (fixed 256px, `useIsMobile` hook unused)
- WebSocket: brak reconnection logic
- `staleTime` default = 0 (every mount refetches)

### 8.15 Frontend — API client

**Plik:** `frontend/src/lib/api.ts` (227 lines)

**Design:** Generic `request<T>()` fetch wrapper, domain-organized `api` object, 19 TypeScript interfaces.

**Problemy:**
- Brak URL encoding dla cluster/topic names (only consumer groups, schemas, connectors encoded)
- Header merge: spread nadpisze `Content-Type` jeśli custom headers
- Brak `AbortSignal` support (no query cancellation)
- `KsqlResponse.data: any` — type safety breach
- Untyped mutation responses (return `Promise<unknown>`)
- No runtime validation (trust server shape)
- No auth header injection hook
- `ClusterOverview` missing from OpenAPI spec

**Testy:** 12 — clusters, brokers, topics, messages, consumerGroups covered. Schemas, connect, ksql, acl: untested.

### 8.16 Frontend — core pages

**Pliki:** `ClustersPage`, `BrokersPage`, `TopicsPage`, `TopicDetailPage`, `TopicMessagesPage`

**Pattern:** `useParams` → `useQuery` → loading/error/data render. Consistent across all pages.

**TopicMessagesPage** (276 lines) — najzłożniejsza strona: browse filters, live tail WebSocket, produce dialog, expandable rows.

**Problemy:**
- `TopicMessagesPage` za duża — wymaga decomposition
- `parseInt` bez radix i NaN fallback
- Produce nie invaliduje messages query
- Brak pagination
- Brak empty state dla tabel (nagłówki bez danych)
- Native `<select>` zamiast shadcn Select
- `window.confirm()` zamiast AlertDialog
- Icon-only buttons bez `aria-label` (accessibility)
- Expandable rows: `onClick` na `<TableRow>` bez keyboard support

**Testy:** 3 (ClustersPage only) — loading, success, error states. 4 remaining pages untested.

### 8.17 Frontend — secondary pages

**Pliki:** Consumer Groups, Schema Registry, Kafka Connect, KSQL, ACL, Dashboard (10 stron)

**Stan:** Wszystkie FULLY IMPLEMENTED (nie placeholdery).

**Problemy:**
- `ConnectorDetailPage`: brak delete confirmation (direct `mutate()`)
- `ConnectorDetailPage`: silent JSON parse error w config editor
- Native `confirm()` zamiast AlertDialog (3 strony)
- Native `<select>` zamiast shadcn Select (ConsumerGroupDetailPage)
- Badge variant helpers zduplikowane w 5+ stronach
- `ACLEntry` type zduplikowany (api.ts + AclPage.tsx)
- SchemaDetailPage: version buttons nie skalują (overflow)
- Brak mutation error display w kilku mutacjach
- Brak success toasts/notifications

**Testy:** 5 (ConsumerGroupsPage only). 9 pozostałych stron: 0 testów.

### 8.18 Frontend — test infrastructure

**Framework:** Vitest v4.0.18, jsdom v28, Testing Library v16.

**Setup:** `@testing-library/jest-dom/vitest` matchers, `globals: true`, `css: true`.

**Mocking:** `vi.mock()` for modules, `vi.fn()` for `global.fetch`. No MSW.

**Problemy:**
- `renderWithProviders` helper zduplikowany (brak shared test utils)
- Verbose mock typing: `(api.x.y as ReturnType<typeof vi.fn>)` — use `vi.mocked()`
- `globals: true` w config ale testy explicite importują z vitest (redundant)
- Brak coverage configuration
- Brak snapshot testing
- Brak WebSocket mock/test library

**Total:** 23 test cases across 4 files. 20+ plików bez testów.

### 8.19 OpenAPI spec

**Plik:** `internal/api/handlers/openapi.yaml`

**4 endpointy brakujące w spec:** dashboard overview, cluster overview, auth status, WebSocket live tail.

**Problemy:**
- `StatusResponse` schema mylące (definiuje `status` + `topic`, ale handlers zwracają różne klucze)
- 9 endpointów bez response body schema
- Zero authentication documentation (no `securitySchemes`)
- Error responses omitted z 13+ endpoints
- `TopicInfo.configs` field missing from spec
- `KsqlResponse.data` typed as `object` (should allow any JSON)
- `omitempty` semantics nie reflected
- Swagger UI loads external CDN assets (unpkg.com)

**Pozytywne:** Dobrze zorganizowane tagi, reusable components ($ref), operation IDs, validation constraints na core endpoints.

### 8.20 Docker & build system

**Pliki:** `Dockerfile`, `docker-compose.yml`, `Makefile`, `scripts/kafka-cluster.sh`, `internal/frontend/embed.go`

**Docker:** 3-stage multi-stage build (node → golang → alpine). Layer caching correct. Final image ~20-40MB.

**Problemy:**
- Container runs as root (brak `USER` instruction)
- Brak `.dockerignore` (COPY sends entire repo including node_modules, .git)
- Brak Docker health checks
- Brak CI/CD pipeline
- `make dev` nie hot-reloaduje Go (plain `go run`)
- Docker Compose: Kafka advertised listener only internal
- Brak restart policy
- Brak OCI labels
- Docker image always tagged `latest`

**Pozytywne:** `go:embed` for single binary, proper CGO_ENABLED=0, good `kafka-cluster.sh` script (3-node KRaft, podman support).

---

## 9. Stan projektu vs plany iteracji

### Implementacja vs commity

| Iteracja | Feature | Committed? |
|----------|---------|:----------:|
| 1 | Core scaffold (clusters, brokers, topics) | Tak |
| 2 | Message browsing, producing, live tail | Tak |
| 3 | Consumer Groups | **Nie** |
| 4 | Schema Registry | **Nie** |
| 5 | Kafka Connect | **Nie** |
| 6 | KSQL | **Nie** |
| 7 | ACL Management | **Nie** |
| 8 | OAuth/OIDC Auth + RBAC | **Nie** (partially wired) |
| 9 | Data Masking | **Nie** (not wired) |
| 10 | Dashboard | **Nie** |

**8 iteracji pracy w jednym dirty working tree** — trudne do review, bisect, rollback.

### Jakość planów

- **Iteracje 1-2**: wyjątkowo szczegółowe (pełny kod, komendy, TDD, ~200+ linii)
- **Iteracje 3-7**: dobre design docs (API specs, modele danych, frontend opisy, ~150 linii)
- **Iteracje 8-10**: szkicowe (~30-70 linii, brak API examples)

### Features zaplanowane ale NIE podpięte

| Feature | Kod istnieje? | Podpięty w app? |
|---------|:---:|:---:|
| Auth middleware | Tak | **Nie** |
| RBAC enforcement | Tak | **Nie** |
| Data masking | Tak | **Nie** |
| Auth routes (login/callback) | Tak | **Nie** |
| Config `BasePath` | Tak | **Nie** |

### Rozbieżności plan vs implementacja

- Master design doc ma inną kolejność iteracji niż actual plans
- Planned component-per-domain structure (`components/clusters/`) → actual flat `pages/` structure
- Planned `/ws/clusters/:id/metrics` WebSocket → nie zaimplementowany
- Frontend auth flow described in plan → brak frontend auth hook/login page
- Planned frontend tests for many pages → only 2 pages tested

---

## 10. Top 10 rekomendacji

### Priorytet: Krytyczny

1. **Podpiąć auth middleware** do route w `router.go` i zarejestrować trasy login/callback/logout/me. Wiring exists — just needs to be connected.

2. **Dodać SPA fallback** — serwuj `index.html` dla ścieżek które nie matchują plików statycznych ani API routes. Bez tego client-side routing nie działa po refresh.

3. **Naprawić CORS** — usunąć `AllowCredentials: true` albo ograniczyć `AllowedOrigins` do konfigurowalnej listy.

### Priorytet: Wysoki

4. **Wyodrębnić `KafkaClient` interface** — umożliwi mock-based testing handlerów (happy-path tests). Single biggest ROI improvement for code quality.

5. **Cachować HTTP clienty** (schema/connect/ksql) zamiast tworzyć nowy `http.Client` na każdy request. Cachuj w handler struct lub registry.

6. **Commitnąć iteracje osobno** — 8 iteracji w jednym dirty tree jest trudne do zarządzania. Commit per iteration z meaningful messages.

7. **Dodać `http.MaxBytesReader`** middleware na POST/PUT endpoints (np. 1MB limit).

### Priorytet: Średni

8. **Zamienić `os.Exit(1)` na error channel** w goroutine servera, żeby deferred cleanup działał.

9. **Dodać config validation** — duplicate names, required fields, valid SASL mechanisms, port range.

10. **Dodać `.dockerignore` i non-root user** do Dockerfile. Dodać health check endpoint (`/api/v1/health`).

---

## Statystyki analizy

| Metryka | Wartość |
|---------|--------|
| Agentów uruchomionych | 21 (równolegle) |
| Plików Go przeanalizowanych | ~40+ |
| Plików React przeanalizowanych | ~35+ |
| Bugów krytycznych | 5 |
| Bugów ważnych | ~35 |
| Bugów mniejszych | ~15 |
| Brakujących features | pagination, binary handling, connection pooling, WS reconnect, CI/CD |

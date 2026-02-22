# Iteration 9: Data Masking

## Scope
Server-side field-level data masking for message values. Applied before messages are sent to the frontend. Configured via YAML rules.

## Architecture
- `internal/masking/engine.go` — Masking engine with glob topic matching and dot-notation field paths
- Applied in message handler (Browse) and WebSocket live tail
- Three masking types: mask (partial), hide (full replacement), hash (SHA256)

### Config
```yaml
data-masking:
  rules:
    - topic-pattern: "*.sensitive.*"
      fields:
        - path: "email"
          type: mask
        - path: "ssn"
          type: hide
        - path: "user.password"
          type: hash
```

### Masking Types
- `mask`: Partial — keep first/last chars, `***` in middle. Email special handling: `j***@e***.com`
- `hide`: Full replacement with `"****"`
- `hash`: SHA256, first 16 hex chars

### Field Paths
- Dot-notation: `"email"`, `"user.email"`, `"data.nested.field"`
- Navigate JSON object hierarchy

## Testing
- 10 engine tests (no rules, no match, mask/hide/hash, nested, non-JSON, wildcards)

## Files
- `internal/masking/engine.go` — new
- `internal/masking/engine_test.go` — new
- `internal/config/config.go` — updated (DataMaskingConfig)

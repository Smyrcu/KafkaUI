package cel

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

// Filter evaluates a CEL expression against Kafka messages.
type Filter struct {
	program cel.Program
}

// NewFilter compiles a CEL expression and returns a Filter.
// Available variables: key (string), value (dyn), headers (map[string,string]),
// partition (int), offset (int), timestamp (timestamp).
func NewFilter(expression string) (*Filter, error) {
	env, err := cel.NewEnv(
		cel.Variable("key", cel.StringType),
		cel.Variable("value", cel.DynType),
		cel.Variable("headers", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("partition", cel.IntType),
		cel.Variable("offset", cel.IntType),
		cel.Variable("timestamp", cel.TimestampType),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, friendlyError(expression, issues.Err())
	}

	if ast.OutputType() != cel.BoolType {
		return nil, fmt.Errorf("filter must return true/false, got %s", ast.OutputType())
	}

	prg, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL program: %w", err)
	}

	return &Filter{program: prg}, nil
}

// Match evaluates the filter against a message record. Returns true if the message matches.
func (f *Filter) Match(msg kafka.MessageRecord) (bool, error) {
	value := parseValue(msg.Value)

	headers := msg.Headers
	if headers == nil {
		headers = map[string]string{}
	}

	activation := map[string]any{
		"key":       msg.Key,
		"value":     value,
		"headers":   headers,
		"partition": int64(msg.Partition),
		"offset":    msg.Offset,
		"timestamp": msg.Timestamp,
	}

	out, _, err := f.program.Eval(activation)
	if err != nil {
		// Missing keys, type mismatches, etc. — treat as no match.
		return false, nil
	}

	if out.Type() != types.BoolType {
		return false, nil
	}

	return out.Value().(bool), nil
}

// friendlyError converts raw CEL compilation errors into user-readable messages.
func friendlyError(expr string, err error) error {
	msg := err.Error()

	// Single = instead of ==
	if strings.Contains(msg, "token recognition error at: '='") ||
		strings.Contains(msg, "extraneous input") {
		if strings.Contains(expr, " = ") && !strings.Contains(expr, "==") && !strings.Contains(expr, "!=") {
			return fmt.Errorf("use == for comparison, not = (e.g. value.status == \"ERROR\")")
		}
	}

	// Undeclared reference
	if strings.Contains(msg, "undeclared reference") {
		for _, v := range []string{"key", "value", "headers", "partition", "offset", "timestamp"} {
			if strings.Contains(msg, "'"+v+"'") {
				break
			}
			if v == "timestamp" {
				// None of the known variables matched — unknown field
				start := strings.Index(msg, "'")
				end := strings.LastIndex(msg, "'")
				if start >= 0 && end > start {
					name := msg[start+1 : end]
					return fmt.Errorf("unknown variable '%s' — available: key, value, headers, partition, offset, timestamp", name)
				}
			}
		}
	}

	// Type mismatch
	if strings.Contains(msg, "found no matching overload") {
		return fmt.Errorf("type mismatch — check that you're comparing the right types (e.g. string to string, number to number)")
	}

	// Unterminated string
	if strings.Contains(msg, "token recognition error at: '\"") {
		return fmt.Errorf("unterminated string — make sure all quotes are closed")
	}

	// Generic syntax error — simplify
	if strings.Contains(msg, "Syntax error") {
		// Extract just the first useful part
		parts := strings.SplitN(msg, "Syntax error:", 2)
		if len(parts) == 2 {
			detail := strings.TrimSpace(parts[1])
			// Take only first sentence
			if idx := strings.Index(detail, " |"); idx > 0 {
				detail = detail[:idx]
			}
			return fmt.Errorf("syntax error: %s", detail)
		}
	}

	return fmt.Errorf("invalid filter: %s", msg)
}

// parseValue tries to parse a JSON string into a map for field access.
// Falls back to the raw string if parsing fails.
func parseValue(raw string) ref.Val {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err == nil {
		return types.DefaultTypeAdapter.NativeToValue(m)
	}
	return types.String(raw)
}


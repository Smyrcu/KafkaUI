package masking

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

// Engine applies data masking rules to message values based on topic patterns.
type Engine struct {
	rules []config.MaskingRule
}

// NewEngine creates a new masking engine from the data masking configuration.
func NewEngine(cfg config.DataMaskingConfig) *Engine {
	return &Engine{rules: cfg.Rules}
}

// MaskMessage applies matching masking rules to the given message value for a topic.
// If no rules match or the value is not valid JSON, the value is returned unchanged.
func (e *Engine) MaskMessage(topic string, value string) string {
	var matchingRules []config.MaskingRule
	for _, rule := range e.rules {
		if e.matchTopic(rule.TopicPattern, topic) {
			matchingRules = append(matchingRules, rule)
		}
	}

	if len(matchingRules) == 0 {
		return value
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return value
	}

	for _, rule := range matchingRules {
		for _, field := range rule.Fields {
			e.applyMask(data, field)
		}
	}

	result, err := json.Marshal(data)
	if err != nil {
		return value
	}

	return string(result)
}

// matchTopic checks if a topic name matches a glob pattern using filepath.Match.
func (e *Engine) matchTopic(pattern, topic string) bool {
	matched, err := filepath.Match(pattern, topic)
	if err != nil {
		return false
	}
	return matched
}

// applyMask applies a single masking field rule to the data map.
func (e *Engine) applyMask(data map[string]any, field config.MaskingField) {
	parent, key, ok := e.getNestedField(data, field.Path)
	if !ok {
		return
	}

	val, exists := parent[key]
	if !exists {
		return
	}

	strVal := fmt.Sprintf("%v", val)

	switch field.Type {
	case "mask":
		parent[key] = e.maskPartial(strVal)
	case "hide":
		parent[key] = "****"
	case "hash":
		parent[key] = e.hashValue(strVal)
	}
}

// getNestedField navigates dot-separated paths in a map and returns the parent map
// and final key. For example, "user.email" navigates to data["user"] and returns
// that map along with the key "email".
func (e *Engine) getNestedField(data map[string]any, path string) (map[string]any, string, bool) {
	parts := strings.Split(path, ".")
	current := data

	for i := 0; i < len(parts)-1; i++ {
		next, ok := current[parts[i]]
		if !ok {
			return nil, "", false
		}

		nextMap, ok := next.(map[string]any)
		if !ok {
			return nil, "", false
		}

		current = nextMap
	}

	return current, parts[len(parts)-1], true
}

// maskPartial performs partial masking on a string value. For email addresses
// (detected by the presence of @), it masks the local part and domain separately,
// preserving the TLD. For other strings longer than 2 characters, it keeps the
// first and last characters and replaces the middle with ***.
func (e *Engine) maskPartial(value string) string {
	if strings.Contains(value, "@") {
		parts := strings.SplitN(value, "@", 2)
		local := parts[0]
		domain := parts[1]

		var maskedLocal string
		if len(local) > 2 {
			maskedLocal = string(local[0]) + "***"
		} else {
			maskedLocal = "***"
		}

		var maskedDomain string
		dotIdx := strings.LastIndex(domain, ".")
		if dotIdx > 0 {
			domainName := domain[:dotIdx]
			tld := domain[dotIdx:]
			if len(domainName) > 1 {
				maskedDomain = string(domainName[0]) + "***" + tld
			} else {
				maskedDomain = "***" + tld
			}
		} else {
			maskedDomain = "***"
		}

		return maskedLocal + "@" + maskedDomain
	}

	if len(value) > 2 {
		return string(value[0]) + "***" + string(value[len(value)-1])
	}

	return "***"
}

// hashValue returns the first 16 hex characters of the SHA256 hash of the value.
func (e *Engine) hashValue(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])[:16]
}

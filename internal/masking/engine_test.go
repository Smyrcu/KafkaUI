package masking

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

func TestEngine_NoRules(t *testing.T) {
	engine := NewEngine(config.DataMaskingConfig{})

	input := `{"name":"Alice","email":"alice@example.com"}`
	output := engine.MaskMessage("any.topic", input)

	if output != input {
		t.Errorf("expected value unchanged, got %s", output)
	}
}

func TestEngine_NoMatchingTopic(t *testing.T) {
	engine := NewEngine(config.DataMaskingConfig{
		Rules: []config.MaskingRule{
			{
				TopicPattern: "sensitive.*",
				Fields: []config.MaskingField{
					{Path: "email", Type: "mask"},
				},
			},
		},
	})

	input := `{"name":"Alice","email":"alice@example.com"}`
	output := engine.MaskMessage("public.data", input)

	if output != input {
		t.Errorf("expected value unchanged for non-matching topic, got %s", output)
	}
}

func TestEngine_MaskEmail(t *testing.T) {
	engine := NewEngine(config.DataMaskingConfig{
		Rules: []config.MaskingRule{
			{
				TopicPattern: "sensitive.*",
				Fields: []config.MaskingField{
					{Path: "email", Type: "mask"},
				},
			},
		},
	})

	input := `{"name":"Alice","email":"alice@example.com"}`
	output := engine.MaskMessage("sensitive.users", input)

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	email, ok := result["email"].(string)
	if !ok {
		t.Fatal("expected email field to be a string")
	}
	if email == "alice@example.com" {
		t.Error("expected email to be masked")
	}
	// Masked email should preserve first char of local part and domain
	if !strings.Contains(email, "***") {
		t.Errorf("expected masked email to contain '***', got %s", email)
	}
	// Name should be unchanged
	if result["name"] != "Alice" {
		t.Error("expected name to be unchanged")
	}
}

func TestEngine_HideField(t *testing.T) {
	engine := NewEngine(config.DataMaskingConfig{
		Rules: []config.MaskingRule{
			{
				TopicPattern: "sensitive.*",
				Fields: []config.MaskingField{
					{Path: "ssn", Type: "hide"},
				},
			},
		},
	})

	input := `{"name":"Bob","ssn":"123-45-6789"}`
	output := engine.MaskMessage("sensitive.users", input)

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	ssn, ok := result["ssn"].(string)
	if !ok {
		t.Fatal("expected ssn field to be a string")
	}
	if ssn != "****" {
		t.Errorf("expected ssn to be '****', got %s", ssn)
	}
	if result["name"] != "Bob" {
		t.Error("expected name to be unchanged")
	}
}

func TestEngine_HashField(t *testing.T) {
	engine := NewEngine(config.DataMaskingConfig{
		Rules: []config.MaskingRule{
			{
				TopicPattern: "sensitive.*",
				Fields: []config.MaskingField{
					{Path: "password", Type: "hash"},
				},
			},
		},
	})

	input := `{"user":"admin","password":"secret123"}`
	output := engine.MaskMessage("sensitive.users", input)

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	password, ok := result["password"].(string)
	if !ok {
		t.Fatal("expected password field to be a string")
	}
	if password == "secret123" {
		t.Error("expected password to be hashed")
	}
	// Should be first 16 hex chars of SHA256
	if len(password) != 16 {
		t.Errorf("expected hashed password to be 16 chars, got %d (%s)", len(password), password)
	}
	for _, c := range password {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("expected hashed password to be hex, got char '%c' in %s", c, password)
			break
		}
	}
	if result["user"] != "admin" {
		t.Error("expected user to be unchanged")
	}
}

func TestEngine_NestedField(t *testing.T) {
	engine := NewEngine(config.DataMaskingConfig{
		Rules: []config.MaskingRule{
			{
				TopicPattern: "sensitive.*",
				Fields: []config.MaskingField{
					{Path: "user.email", Type: "mask"},
				},
			},
		},
	})

	input := `{"user":{"name":"Alice","email":"alice@example.com"}}`
	output := engine.MaskMessage("sensitive.users", input)

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	userObj, ok := result["user"].(map[string]any)
	if !ok {
		t.Fatal("expected user field to be an object")
	}

	email, ok := userObj["email"].(string)
	if !ok {
		t.Fatal("expected user.email field to be a string")
	}
	if email == "alice@example.com" {
		t.Error("expected nested email to be masked")
	}
	if !strings.Contains(email, "***") {
		t.Errorf("expected masked email to contain '***', got %s", email)
	}
	if userObj["name"] != "Alice" {
		t.Error("expected user.name to be unchanged")
	}
}

func TestEngine_NonJSONValue(t *testing.T) {
	engine := NewEngine(config.DataMaskingConfig{
		Rules: []config.MaskingRule{
			{
				TopicPattern: "sensitive.*",
				Fields: []config.MaskingField{
					{Path: "email", Type: "mask"},
				},
			},
		},
	})

	input := "this is plain text, not JSON"
	output := engine.MaskMessage("sensitive.users", input)

	if output != input {
		t.Errorf("expected plain text to be returned unchanged, got %s", output)
	}
}

func TestEngine_MultipleFields(t *testing.T) {
	engine := NewEngine(config.DataMaskingConfig{
		Rules: []config.MaskingRule{
			{
				TopicPattern: "sensitive.*",
				Fields: []config.MaskingField{
					{Path: "email", Type: "mask"},
					{Path: "ssn", Type: "hide"},
				},
			},
		},
	})

	input := `{"name":"Carol","email":"carol@example.com","ssn":"987-65-4321"}`
	output := engine.MaskMessage("sensitive.users", input)

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	email, ok := result["email"].(string)
	if !ok {
		t.Fatal("expected email field to be a string")
	}
	if email == "carol@example.com" {
		t.Error("expected email to be masked")
	}
	if !strings.Contains(email, "***") {
		t.Errorf("expected masked email to contain '***', got %s", email)
	}

	ssn, ok := result["ssn"].(string)
	if !ok {
		t.Fatal("expected ssn field to be a string")
	}
	if ssn != "****" {
		t.Errorf("expected ssn to be '****', got %s", ssn)
	}

	if result["name"] != "Carol" {
		t.Error("expected name to be unchanged")
	}
}

func TestEngine_WildcardTopic(t *testing.T) {
	engine := NewEngine(config.DataMaskingConfig{
		Rules: []config.MaskingRule{
			{
				TopicPattern: "*.sensitive.*",
				Fields: []config.MaskingField{
					{Path: "email", Type: "mask"},
				},
			},
		},
	})

	input := `{"name":"Dave","email":"dave@example.com"}`
	output := engine.MaskMessage("prod.sensitive.users", input)

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	email, ok := result["email"].(string)
	if !ok {
		t.Fatal("expected email field to be a string")
	}
	if email == "dave@example.com" {
		t.Error("expected email to be masked for wildcard topic match")
	}
	if result["name"] != "Dave" {
		t.Error("expected name to be unchanged")
	}
}

func TestEngine_MaskShortString(t *testing.T) {
	engine := NewEngine(config.DataMaskingConfig{
		Rules: []config.MaskingRule{
			{
				TopicPattern: "sensitive.*",
				Fields: []config.MaskingField{
					{Path: "code", Type: "mask"},
				},
			},
		},
	})

	input := `{"id":1,"code":"AB"}`
	output := engine.MaskMessage("sensitive.data", input)

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	code, ok := result["code"].(string)
	if !ok {
		t.Fatal("expected code field to be a string")
	}
	if code != "***" {
		t.Errorf("expected short string to be masked as '***', got %s", code)
	}
}

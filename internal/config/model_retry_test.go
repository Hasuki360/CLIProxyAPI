package config

import (
	"testing"
)

func TestModelRetryParsingAndMatching(t *testing.T) {
	yamlList := []byte(`
model-retry:
  - model: "gpt-6-astra"
    request-retry: 5
    max-retry-interval: 30
  - model: "claude-*"
    request-retry: 2
`)

	cfg, err := ParseConfigBytes(yamlList)
	if err != nil {
		t.Fatalf("ParseConfigBytes(yamlList) error = %v", err)
	}

	if len(cfg.ModelRetry) != 2 {
		t.Fatalf("len(cfg.ModelRetry) = %d, want 2", len(cfg.ModelRetry))
	}

	rule, ok := cfg.FindModelRetry("gpt-6-astra")
	if !ok || rule.RequestRetry != 5 || rule.MaxRetryInterval == nil || *rule.MaxRetryInterval != 30 {
		t.Fatalf("FindModelRetry(gpt-6-astra) = %+v, ok = %v", rule, ok)
	}

	// Case insensitive match
	rule, ok = cfg.FindModelRetry("GPT-6-ASTRA")
	if !ok || rule.RequestRetry != 5 {
		t.Fatalf("FindModelRetry(GPT-6-ASTRA) case-insensitive = %+v, ok = %v", rule, ok)
	}

	// Wildcard match
	rule, ok = cfg.FindModelRetry("claude-3-7-sonnet")
	if !ok || rule.RequestRetry != 2 {
		t.Fatalf("FindModelRetry(claude-3-7-sonnet) wildcard = %+v, ok = %v", rule, ok)
	}

	// Non-matching model
	if cfg.HasModelRetry("gemini-2.5-pro") {
		t.Fatalf("HasModelRetry(gemini-2.5-pro) = true, want false")
	}

	// Map format test
	yamlMap := []byte(`
model-retry:
  gpt-6-astra: 4
  deepseek-reasoner: 2
`)
	cfgMap, err := ParseConfigBytes(yamlMap)
	if err != nil {
		t.Fatalf("ParseConfigBytes(yamlMap) error = %v", err)
	}
	if len(cfgMap.ModelRetry) != 2 {
		t.Fatalf("len(cfgMap.ModelRetry) = %d, want 2", len(cfgMap.ModelRetry))
	}
	ruleMap, ok := cfgMap.FindModelRetry("gpt-6-astra")
	if !ok || ruleMap.RequestRetry != 4 {
		t.Fatalf("FindModelRetry(gpt-6-astra) from map = %+v, ok = %v", ruleMap, ok)
	}
}

package auth

import (
	"testing"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v8/internal/config"
)

func TestRetrySettingsForModel(t *testing.T) {
	mgr := NewManager(nil, nil, nil)
	mgr.requestRetry.Store(2)
	mgr.maxRetryCredentials.Store(5)
	mgr.maxRetryInterval.Store(int64(12 * time.Second))

	cfg := &internalconfig.Config{
		RequestRetry:        2,
		MaxRetryCredentials: 5,
		MaxRetryInterval:    12,
		ModelRetry: internalconfig.ModelRetryRules{
			{
				Model:            "gpt-6-astra",
				RequestRetry:     5,
				MaxRetryInterval: new(int),
			},
		},
	}
	*cfg.ModelRetry[0].MaxRetryInterval = 30
	mgr.SetConfig(cfg)

	// Global default for unmatched model
	retry, creds, wait := mgr.retrySettingsForModel("other-model")
	if retry != 2 || creds != 5 || wait != 12*time.Second {
		t.Fatalf("retrySettingsForModel(other-model) = (%d, %d, %v), want (2, 5, 12s)", retry, creds, wait)
	}

	// Overridden settings for gpt-6-astra
	retryAstra, credsAstra, waitAstra := mgr.retrySettingsForModel("gpt-6-astra")
	if retryAstra != 5 || credsAstra != 5 || waitAstra != 30*time.Second {
		t.Fatalf("retrySettingsForModel(gpt-6-astra) = (%d, %d, %v), want (5, 5, 30s)", retryAstra, credsAstra, waitAstra)
	}
}

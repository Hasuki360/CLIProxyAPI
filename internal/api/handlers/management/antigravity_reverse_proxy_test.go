package management

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestAntigravityReverseProxyManagementHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("port: 8317\n"), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg := &config.Config{
		Port: 8317,
	}

	h := &Handler{
		cfg:            cfg,
		configFilePath: configPath,
	}

	r := gin.New()
	r.GET("/v0/management/antigravity/reverse-proxy", h.GetAntigravityReverseProxy)
	r.PUT("/v0/management/antigravity/reverse-proxy", h.PutAntigravityReverseProxy)

	// 1. Initial GET - default disabled
	req := httptest.NewRequest(http.MethodGet, "/v0/management/antigravity/reverse-proxy", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET returned status %d, want %d", w.Code, http.StatusOK)
	}
	var getResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if getResp["enabled"] != false {
		t.Fatalf("expected enabled=false, got %v", getResp["enabled"])
	}
	if getResp["base-url"] != "https://ag.hasuki.top" {
		t.Fatalf("expected base-url=https://ag.hasuki.top, got %v", getResp["base-url"])
	}

	// 2. PUT to enable reverse proxy
	putPayload := map[string]any{
		"enabled": true,
	}
	putBody, _ := json.Marshal(putPayload)
	req2 := httptest.NewRequest(http.MethodPut, "/v0/management/antigravity/reverse-proxy", bytes.NewReader(putBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("PUT returned status %d, body=%s", w2.Code, w2.Body.String())
	}

	// 3. Verify in-memory state updated
	if !cfg.Antigravity.IsReverseProxyEnabled() {
		t.Fatalf("expected cfg.Antigravity.IsReverseProxyEnabled() to be true")
	}

	// 4. GET again - should be enabled
	req3 := httptest.NewRequest(http.MethodGet, "/v0/management/antigravity/reverse-proxy", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("GET returned status %d", w3.Code)
	}
	var getResp2 map[string]any
	if err := json.Unmarshal(w3.Body.Bytes(), &getResp2); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if getResp2["enabled"] != true {
		t.Fatalf("expected enabled=true, got %v", getResp2["enabled"])
	}

	// 5. PUT to disable with value shorthand
	putPayload2 := map[string]any{
		"value": false,
	}
	putBody2, _ := json.Marshal(putPayload2)
	req4 := httptest.NewRequest(http.MethodPut, "/v0/management/antigravity/reverse-proxy", bytes.NewReader(putBody2))
	req4.Header.Set("Content-Type", "application/json")
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("PUT returned status %d", w4.Code)
	}
	if cfg.Antigravity.IsReverseProxyEnabled() {
		t.Fatalf("expected cfg.Antigravity.IsReverseProxyEnabled() to be false")
	}
}

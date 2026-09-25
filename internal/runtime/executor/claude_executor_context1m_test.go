package executor

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v7/sdk/translator"
	"github.com/tidwall/gjson"
)

type context1MUpstreamCapture struct {
	calls  int
	model  string
	betas  string
	server *httptest.Server
}

func newContext1MUpstream(t *testing.T) *context1MUpstreamCapture {
	t.Helper()
	capture := &context1MUpstreamCapture{}
	capture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capture.calls++
		capture.model = gjson.GetBytes(body, "model").String()
		capture.betas = r.Header.Get("Anthropic-Beta")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","model":"` + capture.model + `","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	t.Cleanup(capture.server.Close)
	return capture
}

func context1MTestAuth(baseURL string) *cliproxyauth.Auth {
	return &cliproxyauth.Auth{Attributes: map[string]string{
		"api_key":  "key-123",
		"base_url": baseURL,
	}}
}

func TestClaudeExecutor_Context1M_SuffixStrippedAndBetaAdded(t *testing.T) {
	cases := []struct {
		name      string
		model     string
		requested string
	}{
		{name: "suffix on execution model", model: "claude-opus-5-5[1m]"},
		{name: "suffix with thinking level", model: "claude-opus-5-5[1m](high)"},
		{name: "suffix only on requested model", model: "claude-opus-5-5", requested: "claude-opus-5-5[1m]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := newContext1MUpstream(t)
			opts := cliproxyexecutor.Options{SourceFormat: sdktranslator.FromString("claude")}
			if tc.requested != "" {
				opts.Metadata = map[string]any{cliproxyexecutor.RequestedModelMetadataKey: tc.requested}
			}
			_, err := NewClaudeExecutor(&config.Config{}).Execute(context.Background(), context1MTestAuth(upstream.server.URL), cliproxyexecutor.Request{
				Model:   tc.model,
				Payload: []byte(`{"model":"` + tc.model + `","messages":[{"role":"user","content":"hi"}]}`),
			}, opts)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if upstream.model != "claude-opus-5-5" {
				t.Fatalf("upstream model = %q, want claude-opus-5-5", upstream.model)
			}
			if !strings.Contains(upstream.betas, claudeContext1MBeta) {
				t.Fatalf("Anthropic-Beta = %q, want %s", upstream.betas, claudeContext1MBeta)
			}
		})
	}
}

func TestClaudeExecutor_Context1M_StreamStripsSuffixAndAddsBeta(t *testing.T) {
	upstream := newContext1MUpstream(t)
	// Only the outgoing request is asserted; the fake upstream's JSON answer is
	// not a valid event stream, so a parse error on the way back is irrelevant.
	result, err := NewClaudeExecutor(&config.Config{}).ExecuteStream(context.Background(), context1MTestAuth(upstream.server.URL), cliproxyexecutor.Request{
		Model:   "claude-sonnet-5[1m]",
		Payload: []byte(`{"model":"claude-sonnet-5[1m]","messages":[{"role":"user","content":"hi"}],"stream":true}`),
	}, cliproxyexecutor.Options{SourceFormat: sdktranslator.FromString("claude"), Stream: true})
	if err == nil && result != nil {
		for range result.Chunks {
		}
	}
	if upstream.calls != 1 {
		t.Fatalf("upstream calls = %d (err = %v), want 1", upstream.calls, err)
	}
	if upstream.model != "claude-sonnet-5" {
		t.Fatalf("upstream model = %q, want claude-sonnet-5", upstream.model)
	}
	if !strings.Contains(upstream.betas, claudeContext1MBeta) {
		t.Fatalf("Anthropic-Beta = %q, want %s", upstream.betas, claudeContext1MBeta)
	}
}

func TestClaudeExecutor_Context1M_NoSuffixNoBeta(t *testing.T) {
	upstream := newContext1MUpstream(t)
	_, err := NewClaudeExecutor(&config.Config{}).Execute(context.Background(), context1MTestAuth(upstream.server.URL), cliproxyexecutor.Request{
		Model:   "claude-opus-5-5",
		Payload: []byte(`{"model":"claude-opus-5-5","messages":[{"role":"user","content":"hi"}]}`),
	}, cliproxyexecutor.Options{SourceFormat: sdktranslator.FromString("claude")})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.Contains(upstream.betas, claudeContext1MBeta) {
		t.Fatalf("Anthropic-Beta = %q, want no %s without [1m]", upstream.betas, claudeContext1MBeta)
	}
}

func TestClaudeExecutor_Context1M_UnsupportedModelRejectedBeforeUpstream(t *testing.T) {
	upstream := newContext1MUpstream(t)
	_, err := NewClaudeExecutor(&config.Config{}).Execute(context.Background(), context1MTestAuth(upstream.server.URL), cliproxyexecutor.Request{
		Model:   "claude-3-7-sonnet-20250219[1m]",
		Payload: []byte(`{"model":"claude-3-7-sonnet-20250219[1m]","messages":[{"role":"user","content":"hi"}]}`),
	}, cliproxyexecutor.Options{SourceFormat: sdktranslator.FromString("claude")})
	var modelErr claudeContext1MModelError
	if !errors.As(err, &modelErr) {
		t.Fatalf("Execute() error = %v, want claudeContext1MModelError", err)
	}
	if modelErr.code != http.StatusBadRequest || !modelErr.IsRequestScoped() {
		t.Fatalf("error = %+v, want request-scoped 400", modelErr)
	}
	if upstream.calls != 0 {
		t.Fatalf("upstream calls = %d, want 0", upstream.calls)
	}
}

func TestKimiExecutor_Context1M_SkipsBeta(t *testing.T) {
	betas, err := NewKimiExecutor(&config.Config{}).context1MBetas(nil, true, "k3")
	if err != nil || len(betas) != 0 {
		t.Fatalf("context1MBetas() = (%v, %v), want no beta for Kimi", betas, err)
	}
}

func TestApplyClaudeHeaders_Context1MAtNativePositionForOAuth(t *testing.T) {
	auth := &cliproxyauth.Auth{
		ID:       "claude-oauth-context1m",
		Metadata: map[string]any{"access_token": "sk-ant-oat-fixture"},
	}
	req, errReq := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages?beta=true", nil)
	if errReq != nil {
		t.Fatalf("NewRequest() error = %v", errReq)
	}
	body := []byte(`{"model":"claude-opus-5-5","messages":[{"role":"user","content":"hi"}]}`)
	if errHeaders := applyClaudeHeaders(req, auth, "sk-ant-oat-fixture", false, []string{claudeContext1MBeta}, body, &config.Config{}, http.Header{}, false); errHeaders != nil {
		t.Fatalf("applyClaudeHeaders() error = %v", errHeaders)
	}
	want := "claude-code-20250219,oauth-2025-04-20,context-1m-2025-08-07,"
	if betas := req.Header.Get("Anthropic-Beta"); !strings.HasPrefix(betas, want) {
		t.Fatalf("Anthropic-Beta = %q, want prefix %q", betas, want)
	}
}

func TestWithClaudeContext1MBeta(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "context-1m-2025-08-07"},
		{"claude-code-20250219,interleaved-thinking-2025-05-14", "claude-code-20250219,context-1m-2025-08-07,interleaved-thinking-2025-05-14"},
		{"claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14", "claude-code-20250219,oauth-2025-04-20,context-1m-2025-08-07,interleaved-thinking-2025-05-14"},
		{"interleaved-thinking-2025-05-14,context-1m-2025-08-07", "context-1m-2025-08-07,interleaved-thinking-2025-05-14"},
		{"claude-code-20250219,context-1m-2025-08-07", "claude-code-20250219,context-1m-2025-08-07"},
	}
	for _, tc := range cases {
		if got := withClaudeContext1MBeta(tc.in); got != tc.want {
			t.Errorf("withClaudeContext1MBeta(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestClaudeModelLacksContext1M(t *testing.T) {
	cases := map[string]bool{
		"claude-opus-5-5":            false,
		"claude-opus-4-8":            false,
		"claude-opus-4-6":            false,
		"claude-sonnet-4-5-20250929": false,
		"claude-sonnet-4-20250514":   false,
		"claude-sonnet-5(high)":      false,
		"claude-fable-5":             false,
		"my-gateway-model":           false,
		"claude-haiku-5":             false,
		"claude-haiku-4-5-20251001":  true,
		"claude-3-5-haiku-20241022":  true,
		"claude-3-7-sonnet-20250219": true,
		"claude-opus-4-5-20251101":   true,
		"claude-opus-4-1-20250805":   true,
		"claude-opus-4-20250514":     true,
		"claude-opus-4-0":            true,
	}
	for model, want := range cases {
		if got := claudeModelLacksContext1M(model); got != want {
			t.Errorf("claudeModelLacksContext1M(%q) = %v, want %v", model, got, want)
		}
	}
}

func TestClassifyClaudeUpstreamError_LongContextEntitlementIsRequestScoped(t *testing.T) {
	cases := []struct {
		status int
		body   string
	}{
		{http.StatusTooManyRequests, `{"type":"error","error":{"type":"rate_limit_error","message":"Extra usage is required for long context requests."}}`},
		{http.StatusBadRequest, `{"type":"error","error":{"type":"invalid_request_error","message":"The long context beta is not yet available for this subscription."}}`},
	}
	for _, tc := range cases {
		err := classifyClaudeUpstreamError(tc.status, http.Header{}, []byte(tc.body))
		var entitlement claudeEntitlementError
		if !errors.As(err, &entitlement) {
			t.Fatalf("classifyClaudeUpstreamError(%d) = %T, want claudeEntitlementError", tc.status, err)
		}
	}
	if err := classifyClaudeUpstreamError(http.StatusBadRequest, http.Header{}, []byte(`{"error":{"message":"prompt is too long: 250000 tokens > 200000 maximum"}}`)); errors.As(err, new(claudeEntitlementError)) {
		t.Fatal("prompt-too-long must stay an ordinary error")
	}
}

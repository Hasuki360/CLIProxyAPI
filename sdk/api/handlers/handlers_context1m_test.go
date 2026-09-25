package handlers

import (
	"context"
	"fmt"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

func TestExecuteModel_Context1MRoutesAsBaseModel(t *testing.T) {
	model := "context1m-route-base-model"
	executor := &modelExecutionCaptureExecutor{}
	handler := newModelExecutionHandler(t, model, executor, &sdkconfig.SDKConfig{})

	for _, requested := range []string{model + "[1m]", model + "[1m](high)"} {
		_, errMsg := handler.ExecuteModel(context.Background(), ModelExecutionRequest{
			EntryProtocol: "claude",
			ExitProtocol:  "claude",
			Model:         requested,
			Body:          []byte(fmt.Sprintf(`{"model":%q}`, requested)),
		})
		if errMsg != nil {
			t.Fatalf("ExecuteModel(%q) error = %+v", requested, errMsg)
		}
		gotReq, gotOpts := executor.captured()
		wantModel := model
		if requested == model+"[1m](high)" {
			wantModel = model + "(high)"
		}
		if gotReq.Model != wantModel {
			t.Fatalf("executor model = %q, want %q", gotReq.Model, wantModel)
		}
		if got := gotOpts.Metadata[coreexecutor.RequestedModelMetadataKey]; got != requested {
			t.Fatalf("requested model metadata = %#v, want %q", got, requested)
		}
	}
}

func TestExecuteModelStream_Context1MRoutesAsBaseModel(t *testing.T) {
	model := "context1m-route-stream-model"
	executor := &modelExecutionCaptureExecutor{
		stream: func(ctx context.Context, auth *coreauth.Auth, req coreexecutor.Request, opts coreexecutor.Options) (*coreexecutor.StreamResult, error) {
			chunks := make(chan coreexecutor.StreamChunk, 1)
			chunks <- coreexecutor.StreamChunk{Payload: []byte("stream-chunk")}
			close(chunks)
			return &coreexecutor.StreamResult{
				Chunks: chunks,
			}, nil
		},
	}
	handler := newModelExecutionHandler(t, model, executor, &sdkconfig.SDKConfig{})

	stream, errMsg := handler.ExecuteModelStream(context.Background(), ModelExecutionRequest{
		EntryProtocol: "claude",
		ExitProtocol:  "claude",
		Model:         model + "[1m]",
		Stream:        true,
		Body:          []byte(fmt.Sprintf(`{"model":%q,"stream":true}`, model+"[1m]")),
	})
	if errMsg != nil {
		t.Fatalf("ExecuteModelStream() error = %+v", errMsg)
	}
	for range stream.Chunks {
	}
	gotReq, gotOpts := executor.captured()
	if gotReq.Model != model {
		t.Fatalf("executor model = %q, want %q", gotReq.Model, model)
	}
	if got := gotOpts.Metadata[coreexecutor.RequestedModelMetadataKey]; got != model+"[1m]" {
		t.Fatalf("requested model metadata = %#v, want %q", got, model+"[1m]")
	}
}

func TestContext1MRouteModel_KeepsRegisteredFullName(t *testing.T) {
	registered := "context1m-registered-alias[1m]"
	registry.GetGlobalRegistry().RegisterClient("context1m-registered-client", "claude", []*registry.ModelInfo{{ID: registered}})
	t.Cleanup(func() {
		registry.GetGlobalRegistry().UnregisterClient("context1m-registered-client")
	})
	handler := NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, coreauth.NewManager(nil, nil, nil))

	if got := handler.context1MRouteModel(registered); got != registered {
		t.Fatalf("context1MRouteModel(%q) = %q, want registered name kept", registered, got)
	}
	if got := handler.context1MRouteModel("context1m-unregistered[1m]"); got != "context1m-unregistered" {
		t.Fatalf("context1MRouteModel(unregistered) = %q, want base model", got)
	}
	if got := handler.context1MRouteModel("context1m-plain"); got != "context1m-plain" {
		t.Fatalf("context1MRouteModel(plain) = %q, want unchanged", got)
	}
}

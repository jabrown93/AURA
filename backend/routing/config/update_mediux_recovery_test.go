package routes_config

import (
	"aura/logging"
	"context"
	"testing"
)

func TestRecoverMediuxAfterTokenCorrectionUsesRuntimeRecoveryCallback(t *testing.T) {
	oldCallback := RecoverMediuxRuntimeState
	t.Cleanup(func() { RecoverMediuxRuntimeState = oldCallback })

	calls := 0
	RecoverMediuxRuntimeState = func(context.Context) logging.LogErrorInfo {
		calls++
		return logging.LogErrorInfo{Message: "items preload failed"}
	}

	Err := recoverMediuxAfterTokenCorrection(context.Background(), true)
	if Err.Message != "items preload failed" {
		t.Fatalf("recovery error = %q, want %q", Err.Message, "items preload failed")
	}
	if calls != 1 {
		t.Fatalf("recovery calls = %d, want 1", calls)
	}

	Err = recoverMediuxAfterTokenCorrection(context.Background(), false)
	if Err.Message != "" {
		t.Fatalf("unchanged token recovery error = %q, want none", Err.Message)
	}
	if calls != 1 {
		t.Fatalf("recovery calls after unchanged token = %d, want 1", calls)
	}
}

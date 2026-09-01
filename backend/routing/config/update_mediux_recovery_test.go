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

	// A failed recovery must not surface to the caller: the saved token is still
	// valid, and the background recheck loop retries the rebuild.
	recoverMediuxAfterTokenCorrection(context.Background(), true)
	if calls != 1 {
		t.Fatalf("recovery calls = %d, want 1", calls)
	}

	recoverMediuxAfterTokenCorrection(context.Background(), false)
	if calls != 1 {
		t.Fatalf("recovery calls after unchanged token = %d, want 1", calls)
	}
}

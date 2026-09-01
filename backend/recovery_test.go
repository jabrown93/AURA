package main

import (
	"aura/config"
	"aura/logging"
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestRecoverMediuxRuntimeStateRunsEveryRequiredStage(t *testing.T) {
	oldPreloadUsers := preloadMediuxUsers
	oldPreloadItems := preloadMediuxItemsWithSets
	oldRefreshLibrary := refreshLibraryItems
	t.Cleanup(func() {
		preloadMediuxUsers = oldPreloadUsers
		preloadMediuxItemsWithSets = oldPreloadItems
		refreshLibraryItems = oldRefreshLibrary
	})

	var calls []string
	var refreshForced bool
	preloadMediuxUsers = func(context.Context) logging.LogErrorInfo {
		calls = append(calls, "users")
		return logging.LogErrorInfo{}
	}
	preloadMediuxItemsWithSets = func(context.Context) logging.LogErrorInfo {
		calls = append(calls, "items")
		return logging.LogErrorInfo{}
	}
	refreshLibraryItems = func(_ context.Context, force bool) bool {
		calls = append(calls, "library")
		refreshForced = force
		return true
	}

	Err := recoverMediuxRuntimeState(recoveryTestContext())

	if Err.Message != "" {
		t.Fatalf("recoverMediuxRuntimeState() error = %q, want none", Err.Message)
	}
	if want := []string{"users", "items", "library"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("recovery calls = %v, want %v", calls, want)
	}
	if !refreshForced {
		t.Fatal("recovery library refresh was not forced")
	}
}

func TestRecoverMediuxRuntimeStateReportsEveryStageFailure(t *testing.T) {
	oldPreloadUsers := preloadMediuxUsers
	oldPreloadItems := preloadMediuxItemsWithSets
	oldRefreshLibrary := refreshLibraryItems
	t.Cleanup(func() {
		preloadMediuxUsers = oldPreloadUsers
		preloadMediuxItemsWithSets = oldPreloadItems
		refreshLibraryItems = oldRefreshLibrary
	})

	tests := []struct {
		name                string
		usersErr            logging.LogErrorInfo
		itemsErr            logging.LogErrorInfo
		libraryOK           bool
		wantSubstring       string
		wantMediuxReachable bool
	}{
		{name: "users preload", usersErr: logging.LogErrorInfo{Message: "users failed"}, libraryOK: true, wantSubstring: "users failed"},
		{name: "items preload", itemsErr: logging.LogErrorInfo{Message: "items failed"}, libraryOK: true, wantSubstring: "items failed"},
		{name: "library refresh", libraryOK: false, wantSubstring: "library", wantMediuxReachable: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			preloadMediuxUsers = func(context.Context) logging.LogErrorInfo {
				calls = append(calls, "users")
				return tt.usersErr
			}
			preloadMediuxItemsWithSets = func(context.Context) logging.LogErrorInfo {
				calls = append(calls, "items")
				return tt.itemsErr
			}
			refreshLibraryItems = func(context.Context, bool) bool {
				calls = append(calls, "library")
				return tt.libraryOK
			}

			config.MediuxReachable = true
			Err := recoverMediuxRuntimeState(recoveryTestContext())

			if Err.Message == "" {
				t.Fatal("recoverMediuxRuntimeState() returned no error")
			}
			if want := []string{"users", "items", "library"}; !reflect.DeepEqual(calls, want) {
				t.Fatalf("recovery calls = %v, want %v", calls, want)
			}
			if !strings.Contains(Err.Message, tt.wantSubstring) {
				t.Fatalf("recovery error = %q, want substring %q", Err.Message, tt.wantSubstring)
			}
			if config.MediuxReachable != tt.wantMediuxReachable {
				t.Fatalf("MediuxReachable = %v, want %v", config.MediuxReachable, tt.wantMediuxReachable)
			}
		})
	}
}

func recoveryTestContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "MediUX Recovery Test")
	action := ld.AddAction("Recovering", logging.LevelInfo)
	return logging.WithCurrentAction(ctx, action)
}

func TestHandleMediuxRecheckStopsOnlyAfterAcceptedTokenAndRecovery(t *testing.T) {
	preservePreFlightState(t)

	tests := []struct {
		name            string
		result          mediuxTokenResult
		recoveryErr     logging.LogErrorInfo
		wantCalls       int
		wantRecoveryErr bool
	}{
		{name: "accepted and recovered", result: mediuxTokenAccepted, wantCalls: 1},
		{name: "accepted but recovery failed", result: mediuxTokenAccepted, recoveryErr: logging.LogErrorInfo{Message: "preload failed"}, wantCalls: 1, wantRecoveryErr: true},
		{name: "rejected", result: mediuxTokenRejected, wantCalls: 0},
		{name: "unreachable", result: mediuxUnreachable, wantCalls: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			got, recoveryErr := handleMediuxRecheck(context.Background(), func(context.Context) mediuxTokenResult {
				config.MediuxValid = tt.result == mediuxTokenAccepted
				config.MediuxReachable = tt.result != mediuxUnreachable
				return tt.result
			}, func(context.Context) logging.LogErrorInfo {
				calls++
				return tt.recoveryErr
			})

			if got != tt.result {
				t.Fatalf("handleMediuxRecheck() = %v, want %v", got, tt.result)
			}
			if calls != tt.wantCalls {
				t.Fatalf("recovery calls = %d, want %d", calls, tt.wantCalls)
			}
			if (recoveryErr.Message != "") != tt.wantRecoveryErr {
				t.Fatalf("recovery error = %q, want error %v", recoveryErr.Message, tt.wantRecoveryErr)
			}
		})
	}
}

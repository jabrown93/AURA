package main

import (
	"aura/config"
	"context"
	"reflect"
	"testing"
)

func TestRecoverMediuxRuntimeStateRebuildsOnlyMediuxDerivedCaches(t *testing.T) {
	oldPreloadUsers := preloadMediuxUsers
	oldPreloadItems := preloadMediuxItemsWithSets
	oldRefreshLibrary := refreshLibraryItems
	t.Cleanup(func() {
		preloadMediuxUsers = oldPreloadUsers
		preloadMediuxItemsWithSets = oldPreloadItems
		refreshLibraryItems = oldRefreshLibrary
	})

	var calls []string
	preloadMediuxUsers = func(context.Context) { calls = append(calls, "users") }
	preloadMediuxItemsWithSets = func(context.Context) { calls = append(calls, "items") }
	refreshLibraryItems = func(context.Context, bool) bool {
		calls = append(calls, "library")
		return true
	}

	recoverMediuxRuntimeState(context.Background())

	if want := []string{"users", "items", "library"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("recovery calls = %v, want %v", calls, want)
	}
}

func TestHandleMediuxRecheckRecoversAcceptedTokenOnly(t *testing.T) {
	preservePreFlightState(t)

	tests := []struct {
		name      string
		result    mediuxTokenResult
		wantCalls int
	}{
		{name: "accepted", result: mediuxTokenAccepted, wantCalls: 1},
		{name: "rejected", result: mediuxTokenRejected, wantCalls: 0},
		{name: "unreachable", result: mediuxUnreachable, wantCalls: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			got := handleMediuxRecheck(context.Background(), func(context.Context) mediuxTokenResult {
				config.MediuxValid = tt.result == mediuxTokenAccepted
				config.MediuxReachable = tt.result != mediuxUnreachable
				return tt.result
			}, func(context.Context) { calls++ })

			if got != tt.result {
				t.Fatalf("handleMediuxRecheck() = %v, want %v", got, tt.result)
			}
			if calls != tt.wantCalls {
				t.Fatalf("recovery calls = %d, want %d", calls, tt.wantCalls)
			}
		})
	}
}

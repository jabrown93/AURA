package routes_jobs

import (
	"aura/jobs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunJobReturnsConflictWhenJobIsBusy(t *testing.T) {
	previous := triggerJob
	triggerJob = func(string, string) error { return jobs.ErrJobBusy }
	t.Cleanup(func() { triggerJob = previous })

	request := httptest.NewRequest(http.MethodPost, "/api/jobs?job_name=Download+Queue+Processing+Job&job_id=job-id", nil)
	response := httptest.NewRecorder()

	RunJob(response, request)

	if response.Code != http.StatusConflict {
		t.Errorf("RunJob() status = %d, want %d", response.Code, http.StatusConflict)
	}
	if !strings.Contains(response.Body.String(), "already running") {
		t.Errorf("RunJob() body = %q, want busy message", response.Body.String())
	}
}

package jobs

import (
	"aura/database"
	"testing"

	"github.com/robfig/cron/v3"
)

// TestPostgresDisablesSQLiteBackupJob checks scheduling, discovery and manual invocation.
func TestPostgresDisablesSQLiteBackupJob(t *testing.T) {
	oldDB, oldCron := database.Client, c
	t.Cleanup(func() { database.Client, c = oldDB, oldCron })
	database.Client, c = &database.PostgresDB{}, cron.New()
	for _, job := range GetListOfJobs() {
		if job.ID == string(JobKeyBackupDatabase) {
			t.Fatal("PostgreSQL advertised SQLite backup job")
		}
	}
	if err := StartBackupDatabaseJob(); err != nil {
		t.Fatal(err)
	}
	if id := jobEntryIDs[JobKeyBackupDatabase]; id != 0 {
		t.Fatal("PostgreSQL scheduled SQLite backup job")
	}
	if err := TriggerJob(JobKeyBackupDatabase); err == nil {
		t.Fatal("PostgreSQL accepted manual SQLite backup job")
	}
}

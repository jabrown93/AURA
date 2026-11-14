package main

import (
	"aura/database"
	"aura/database/migration"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
)

// forkDatabaseCommand runs offline maintenance before any API or worker can access database globals.
func forkDatabaseCommand(ctx context.Context, args []string, stdout, stderr io.Writer) (bool, int) {
	if len(args) == 0 || args[0] != "db" {
		return false, 0
	}
	if len(args) < 2 || args[1] != "import-sqlite" {
		fmt.Fprintln(stderr, "usage: aura db import-sqlite --source PATH --confirm-app-stopped")
		return true, 2
	}
	flags := flag.NewFlagSet("aura db import-sqlite", flag.ContinueOnError)
	flags.SetOutput(stderr)
	source := flags.String("source", "", "SQLite file to snapshot and import; source remains unchanged")
	stopped := flags.Bool("confirm-app-stopped", false, "confirm all AURA instances are stopped during import")
	if err := flags.Parse(args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return true, 0
		}
		return true, 2
	}
	if *source == "" || !*stopped || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "--source PATH and --confirm-app-stopped are required; stop all AURA instances first")
		return true, 2
	}
	client, info := database.NewDatabaseClient()
	if info.Message != "" {
		fmt.Fprintln(stderr, info.Message)
		return true, 1
	}
	target, ok := client.(*database.PostgresDB)
	if !ok {
		fmt.Fprintln(stderr, "import requires AURA_DB_DRIVER=postgres and AURA_POSTGRES_DSN")
		return true, 2
	}
	defer target.Close(ctx)
	report, err := migration.ImportSQLiteOffline(ctx, *source, target)
	if err != nil {
		fmt.Fprintln(stderr, "SQLite import failed:", err)
		return true, 1
	}
	var rows int64
	for _, count := range report.Tables {
		rows += count
	}
	fmt.Fprintf(stdout, "Imported %d rows across %d tables; reset %d interrupted queue jobs. Source unchanged.\n", rows, len(report.Tables), report.ResetQueueJobs)
	return true, 0
}

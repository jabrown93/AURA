package database

import (
	"aura/config"
	"aura/logging"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (s *SQliteDB) Backup(ctx context.Context, currentVersion, newVersion int) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Backing up SQLite Database", logging.LevelInfo)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// Get the DB path
	dbPath := config.DatabasePath()

	// Check if the database file exists
	// If it doesn't exist, there's nothing to back up
	// This can happen on first run
	// So we just return
	_, statErr := os.Stat(dbPath)
	if os.IsNotExist(statErr) {
		return Err
	} else if statErr != nil {
		logAction.SetError("Failed to stat database file for backup",
			"Ensure the database path is accessible.",
			map[string]any{
				"error": statErr.Error(),
				"path":  dbPath,
			})
		return *logAction.Error
	}

	// Create a backup file path with version and timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s_backup_v%d_to_v%d_%s.db", strings.TrimSuffix(dbPath, path.Ext(dbPath)), currentVersion, newVersion, timestamp)

	actionBackupFile := logAction.AddSubAction("Backing up database file", logging.LevelTrace)
	if err := s.vacuumInto(ctx, backupPath); err != nil {
		actionBackupFile.SetError("Failed to back up database file",
			"Ensure the destination path is accessible and writable, and that there is sufficient disk space.",
			map[string]any{
				"error":       err.Error(),
				"source":      dbPath,
				"destination": backupPath,
			})
		return *actionBackupFile.Error
	}
	actionBackupFile.Complete()

	return Err
}

// vacuumInto writes a copy of the database to path.
// VACUUM INTO is used instead of copying the database file.
// In WAL mode, committed writes can still be in the WAL file, and copying the file while
// another connection writes can produce a torn copy. VACUUM INTO writes a consistent
// snapshot that includes the WAL contents, even while other connections are writing.
func (s *SQliteDB) vacuumInto(ctx context.Context, path string) error {
	_, err := s.conn.ExecContext(ctx, "VACUUM INTO ?", path)
	return err
}

// DatabaseBackup is a backup of the database in the backups folder
type DatabaseBackup struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"` // Bytes
	CreatedAt time.Time `json:"created_at"`
}

const backupTimeFormat = "2006-01-02_15-04-05"

// Only one backup is made at a time
var backupMu sync.Mutex

// BackupsFolder returns the folder the database backups are saved in, next to the database
func BackupsFolder() string {
	return filepath.Join(filepath.Dir(config.DatabasePath()), "backups")
}

// backupBaseName returns the database file name without its extension, e.g. AURA
func backupBaseName() string {
	name := filepath.Base(config.DatabasePath())
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// backupNamePattern matches the names of the backups aura makes, so other files in the folder are never deleted
func backupNamePattern() *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(backupBaseName()) + `_(\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2})(?:_(\d+))?\.db$`)
}

// CreateBackup writes a backup of the database to the backups folder, then deletes the oldest backups so only keep are left
func (s *SQliteDB) CreateBackup(ctx context.Context, keep int) (backup DatabaseBackup, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Backing Up Database", logging.LevelInfo)
	defer logAction.Complete()

	backupMu.Lock()
	defer backupMu.Unlock()

	folder := BackupsFolder()
	if err := os.MkdirAll(folder, 0o755); err != nil {
		logAction.SetError("Could not create the backups folder", "Make sure the config folder can be written to", map[string]any{"error": err.Error(), "folder": folder})
		return backup, *logAction.Error
	}

	now := time.Now()
	name := nextBackupName(folder, now)
	backupPath := filepath.Join(folder, name)

	if err := s.vacuumInto(ctx, backupPath); err != nil {
		_ = os.Remove(backupPath)
		logAction.SetError("Could not back up the database", "Make sure the config folder can be written to and has enough free space", map[string]any{"error": err.Error(), "path": backupPath})
		return backup, *logAction.Error
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		logAction.SetError("Could not read the new backup", "", map[string]any{"error": err.Error(), "path": backupPath})
		return backup, *logAction.Error
	}
	backup = DatabaseBackup{Name: name, Size: info.Size(), CreatedAt: now}
	logAction.AppendResult("backup", name)
	logAction.AppendResult("size", info.Size())

	deleted, err := pruneBackups(keep)
	if err != nil {
		logAction.AppendWarning("delete_old_backups_error", err.Error())
	}
	if len(deleted) > 0 {
		logAction.AppendResult("deleted_backups", deleted)
	}
	return backup, logging.LogErrorInfo{}
}

// ListBackups returns the backups in the backups folder, newest first
func ListBackups() ([]DatabaseBackup, error) {
	entries, err := os.ReadDir(BackupsFolder())
	if os.IsNotExist(err) {
		return []DatabaseBackup{}, nil
	}
	if err != nil {
		return nil, err
	}

	type sortableBackup struct {
		backup DatabaseBackup
		number int
	}
	pattern := backupNamePattern()
	found := []sortableBackup{}
	for _, entry := range entries {
		match := pattern.FindStringSubmatch(entry.Name())
		if entry.IsDir() || match == nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		createdAt, err := time.ParseInLocation(backupTimeFormat, match[1], time.Local)
		if err != nil {
			continue
		}
		number := 1
		if match[2] != "" {
			number, _ = strconv.Atoi(match[2])
		}
		found = append(found, sortableBackup{
			backup: DatabaseBackup{Name: entry.Name(), Size: info.Size(), CreatedAt: createdAt},
			number: number,
		})
	}

	slices.SortFunc(found, func(a, b sortableBackup) int {
		if c := b.backup.CreatedAt.Compare(a.backup.CreatedAt); c != 0 {
			return c
		}
		return b.number - a.number
	})
	backups := make([]DatabaseBackup, 0, len(found))
	for _, f := range found {
		backups = append(backups, f.backup)
	}
	return backups, nil
}

// pruneBackups deletes the oldest backups so only keep are left
func pruneBackups(keep int) (deleted []string, err error) {
	if keep < 1 {
		keep = 1
	}
	backups, err := ListBackups()
	if err != nil || len(backups) <= keep {
		return nil, err
	}
	var errs []string
	for _, backup := range backups[keep:] {
		if err := os.Remove(filepath.Join(BackupsFolder(), backup.Name)); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		deleted = append(deleted, backup.Name)
	}
	if len(errs) > 0 {
		return deleted, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return deleted, nil
}

// nextBackupName returns the name of a new backup. Backups made in the same second get a higher number than the ones before
func nextBackupName(folder string, now time.Time) string {
	stamp := now.Format(backupTimeFormat)
	highest := 0
	entries, _ := os.ReadDir(folder)
	pattern := backupNamePattern()
	for _, entry := range entries {
		match := pattern.FindStringSubmatch(entry.Name())
		if match == nil || match[1] != stamp {
			continue
		}
		number := 1
		if match[2] != "" {
			number, _ = strconv.Atoi(match[2])
		}
		highest = max(highest, number)
	}
	if highest == 0 {
		return fmt.Sprintf("%s_%s.db", backupBaseName(), stamp)
	}
	return fmt.Sprintf("%s_%s_%d.db", backupBaseName(), stamp, highest+1)
}

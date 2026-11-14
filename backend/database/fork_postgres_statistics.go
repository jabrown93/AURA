package database

import (
	"aura/logging"
	"context"
	"fmt"
	"time"
)

// GetDownloadEventTotals uses native PostgreSQL timestamps without SQLite date functions.
func (p *PostgresDB) GetDownloadEventTotals(ctx context.Context, since time.Time) (totals []DownloadEventTotal, info logging.LogErrorInfo) {
	rows, err := p.conn.QueryContext(ctx, `SELECT kind,media_item_type='collection',COUNT(*),SUM(images_succeeded),SUM(images_failed),SUM(bytes),SUM(estimated)
FROM DownloadEvents WHERE created_at >= $1 GROUP BY kind,media_item_type='collection'`, since)
	if err != nil {
		return nil, postgresError("Failed to query download totals", err)
	}
	defer rows.Close()
	for rows.Next() {
		var total DownloadEventTotal
		if err := rows.Scan(&total.Kind, &total.Collection, &total.Events, &total.ImagesSucceeded, &total.ImagesFailed, &total.Bytes, &total.Estimated); err != nil {
			return nil, postgresError("Failed to scan download totals", err)
		}
		totals = append(totals, total)
	}
	return totals, postgresError("Failed to read download totals", rows.Err())
}

// GetDownloadEventTimeline groups into UTC calendar buckets with Monday-start weeks.
func (p *PostgresDB) GetDownloadEventTimeline(ctx context.Context, since time.Time, period string) (buckets []DownloadEventBucket, info logging.LogErrorInfo) {
	if period != "week" && period != "month" {
		period = "day"
	}
	rows, err := p.conn.QueryContext(ctx, `SELECT to_char(date_trunc($2,created_at AT TIME ZONE 'UTC'),'YYYY-MM-DD') AS bucket,
kind,media_item_type='collection',COUNT(*),SUM(images_succeeded),SUM(images_failed),SUM(bytes)
FROM DownloadEvents WHERE created_at >= $1 GROUP BY bucket,kind,media_item_type='collection' ORDER BY bucket`, since, period)
	if err != nil {
		return nil, postgresError("Failed to query download timeline", err)
	}
	defer rows.Close()
	for rows.Next() {
		var bucket DownloadEventBucket
		if err := rows.Scan(&bucket.Date, &bucket.Kind, &bucket.Collection, &bucket.Events, &bucket.ImagesSucceeded, &bucket.ImagesFailed, &bucket.Bytes); err != nil {
			return nil, postgresError("Failed to scan download timeline", err)
		}
		buckets = append(buckets, bucket)
	}
	return buckets, postgresError("Failed to read download timeline", rows.Err())
}

// GetDownloadEventGroups permits only the same two grouping columns as SQLite.
func (p *PostgresDB) GetDownloadEventGroups(ctx context.Context, since time.Time, groupBy string, limit int) (groups []DownloadEventGroup, info logging.LogErrorInfo) {
	column := map[string]string{"creator": "set_creator", "source": "source"}[groupBy]
	if column == "" {
		return nil, logging.LogErrorInfo{Message: fmt.Sprintf("Unknown download group '%s'", groupBy)}
	}
	rows, err := p.conn.QueryContext(ctx, fmt.Sprintf(`SELECT %[1]s,SUM(CASE WHEN kind='download' AND media_item_type<>'collection' THEN 1 ELSE 0 END),SUM(images_succeeded)
FROM DownloadEvents WHERE created_at >= $1 AND kind<>'saved' AND %[1]s<>'' GROUP BY %[1]s ORDER BY 2 DESC,3 DESC LIMIT $2`, column), since, limit)
	if err != nil {
		return nil, postgresError("Failed to query download groups", err)
	}
	defer rows.Close()
	for rows.Next() {
		var group DownloadEventGroup
		if err := rows.Scan(&group.Name, &group.Sets, &group.Images); err != nil {
			return nil, postgresError("Failed to scan download group", err)
		}
		groups = append(groups, group)
	}
	return groups, postgresError("Failed to read download groups", rows.Err())
}

// GetDownloadEventImageTypes expands native JSONB counts, not SQLite json_each values.
func (p *PostgresDB) GetDownloadEventImageTypes(ctx context.Context, since time.Time) (types []NameCount, info logging.LogErrorInfo) {
	rows, err := p.conn.QueryContext(ctx, `SELECT image_type.key,SUM(image_type.value::bigint)
FROM DownloadEvents CROSS JOIN LATERAL jsonb_each_text(image_types) image_type
WHERE created_at >= $1 AND kind IN ('download','update') GROUP BY image_type.key ORDER BY 2 DESC`, since)
	if err != nil {
		return nil, postgresError("Failed to query image type counts", err)
	}
	defer rows.Close()
	for rows.Next() {
		var count NameCount
		if err := rows.Scan(&count.Name, &count.Count); err != nil {
			return nil, postgresError("Failed to scan image type counts", err)
		}
		types = append(types, count)
	}
	return types, postgresError("Failed to read image type counts", rows.Err())
}

// GetRedownloadCount preserves event-ID ordering when timestamps collide.
func (p *PostgresDB) GetRedownloadCount(ctx context.Context, since time.Time) (count int, info logging.LogErrorInfo) {
	err := p.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM DownloadEvents event
WHERE event.created_at >= $1 AND event.kind='download' AND event.media_item_type<>'collection'
AND EXISTS (SELECT 1 FROM DownloadEvents earlier WHERE earlier.kind='download' AND earlier.tmdb_id=event.tmdb_id
AND earlier.library_title=event.library_title AND earlier.edition=event.edition AND earlier.set_id=event.set_id AND earlier.id<event.id)`, since).Scan(&count)
	return count, postgresError("Failed to count redownloads", err)
}

// GetFailureReasons counts historical image errors, including unknown reasons.
func (p *PostgresDB) GetFailureReasons(ctx context.Context, limit int) (reasons []NameCount, info logging.LogErrorInfo) {
	rows, err := p.conn.QueryContext(ctx, `SELECT COALESCE(NULLIF(failed.value->>'failure_reason',''),'Unknown'),COUNT(*)
FROM DownloadHistory CROSS JOIN LATERAL jsonb_array_elements(failed_images) failed GROUP BY 1 ORDER BY 2 DESC LIMIT $1`, limit)
	if err != nil {
		return nil, postgresError("Failed to query failure reasons", err)
	}
	defer rows.Close()
	for rows.Next() {
		var reason NameCount
		if err := rows.Scan(&reason.Name, &reason.Count); err != nil {
			return nil, postgresError("Failed to scan failure reason", err)
		}
		reasons = append(reasons, reason)
	}
	return reasons, postgresError("Failed to read failure reasons", rows.Err())
}

// UpsertStatisticsSnapshot replaces only the requested day's snapshot.
func (p *PostgresDB) UpsertStatisticsSnapshot(ctx context.Context, date, data string) logging.LogErrorInfo {
	_, err := p.conn.ExecContext(ctx, `INSERT INTO StatisticsSnapshots (snapshot_date,data,updated_at) VALUES ($1,$2,CURRENT_TIMESTAMP)
ON CONFLICT (snapshot_date) DO UPDATE SET data=EXCLUDED.data,updated_at=EXCLUDED.updated_at`, date, data)
	return postgresError("Failed to save statistics snapshot", err)
}

// GetStatisticsSnapshots returns canonical JSONB as API-compatible JSON text.
func (p *PostgresDB) GetStatisticsSnapshots(ctx context.Context, sinceDate string) (snapshots []StatisticsSnapshot, info logging.LogErrorInfo) {
	rows, err := p.conn.QueryContext(ctx, "SELECT snapshot_date,data FROM StatisticsSnapshots WHERE snapshot_date >= $1 ORDER BY snapshot_date", sinceDate)
	if err != nil {
		return nil, postgresError("Failed to query statistics snapshots", err)
	}
	defer rows.Close()
	for rows.Next() {
		var snapshot StatisticsSnapshot
		if err := rows.Scan(&snapshot.Date, &snapshot.Data); err != nil {
			return nil, postgresError("Failed to scan statistics snapshot", err)
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, postgresError("Failed to read statistics snapshots", rows.Err())
}

// CollectionCounts counts identities without delimiter collisions in library names.
func (p *PostgresDB) CollectionCounts(ctx context.Context) (withImages, savedCollectionSets int, info logging.LogErrorInfo) {
	err := p.conn.QueryRowContext(ctx, `SELECT
(SELECT COUNT(DISTINCT (tmdb_id,library_title)) FROM DownloadEvents WHERE media_item_type='collection' AND images_succeeded>0),
(SELECT COUNT(*) FROM SavedItems si JOIN PosterSets ps ON ps.id=si.poster_set_id WHERE ps.type='collection')`).Scan(&withImages, &savedCollectionSets)
	return withImages, savedCollectionSets, postgresError("Failed to count collections", err)
}

// CountDownloadHistory uses the PostgreSQL connection selected at startup.
func (p *PostgresDB) CountDownloadHistory(ctx context.Context) (count int, info logging.LogErrorInfo) {
	err := p.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM DownloadHistory").Scan(&count)
	return count, postgresError("Failed to count download history", err)
}

package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

const postgresMediaJSON = `jsonb_build_object(
 'tmdb_id',mi.tmdb_id,'library_title',mi.library_title,'edition',mi.edition,
 'rating_key',mi.rating_key,'type',mi.type,'title',mi.title,'year',mi.year,
 'added_at',mi.added_at,'updated_at',mi.updated_at,'released_at',mi.released_at,
 'latest_episode_added_at',mi.latest_episode_added_at,'content_rating',mi.content_rating,'summary',mi.summary,'guids',mi.guids,
 'movie',CASE WHEN mi.type='movie' THEN (SELECT jsonb_build_object('file',jsonb_build_object('path',mv.path,'size',mv.size,'duration',mv.duration)) FROM Movies mv WHERE mv.media_item_id=mi.id) END,
 'series',CASE WHEN mi.type='show' THEN (SELECT jsonb_build_object('season_count',sr.season_count,'episode_count',sr.episode_count,'location',sr.location,
 'seasons',COALESCE((SELECT jsonb_agg(jsonb_build_object('rating_key',sn.rating_key,'season_number',sn.season_number,
 'episodes',COALESCE((SELECT jsonb_agg(jsonb_build_object('rating_key',ep.rating_key,'episode_number',ep.episode_number,'season_number',sn.season_number,'title',ep.title,
 'file',jsonb_build_object('path',ep.path,'size',ep.size,'duration',ep.duration)) ORDER BY ep.episode_number) FROM Episodes ep WHERE ep.season_id=sn.id),'[]'::jsonb)) ORDER BY sn.season_number)
 FROM Seasons sn WHERE sn.series_id=sr.id),'[]'::jsonb)) FROM Series sr WHERE sr.media_item_id=mi.id) END)`

const postgresSetsJSON = `COALESCE((SELECT jsonb_agg(jsonb_build_object(
 'id',ps.set_id,'title',ps.title,'type',ps.type,'user_created',ps."user",
 'date_created',ps.date_created,'date_updated',ps.date_updated,'last_downloaded',si.last_downloaded,
 'selected_types',jsonb_build_object('poster',si.poster_selected=1,'backdrop',si.backdrop_selected=1,'season_poster',si.season_poster_selected=1,'special_season_poster',si.special_season_poster_selected=1,'titlecard',si.titlecard_selected=1),
 'auto_download',si.autodownload=1,'auto_add_new_collection_items',si.auto_add_new_collection_items=1,'force_preload_missing',si.force_preload_missing=1,'no_auto_upgrade',si.no_auto_upgrade=1,
 'images',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',im.image_id,'type',im.image_type,'modified',im.image_last_updated,'season_number',im.image_season_number,'episode_number',im.image_episode_number) ORDER BY im.id)
 FROM ImageFiles im WHERE im.poster_set_id=ps.id AND im.item_tmdb_id=mi.tmdb_id),'[]'::jsonb)) ORDER BY ps.id)
 FROM SavedItems si JOIN PosterSets ps ON ps.id=si.poster_set_id WHERE si.tmdb_id=mi.tmdb_id AND si.library_title=mi.library_title AND si.edition=mi.edition),'[]'::jsonb)`

const postgresSavedBase = `WITH base AS (SELECT mi.*,
 (SELECT count(*) FROM SavedItems si WHERE si.tmdb_id=mi.tmdb_id AND si.library_title=mi.library_title AND si.edition=mi.edition) AS set_count,
 (SELECT max(last_downloaded) FROM SavedItems si WHERE si.tmdb_id=mi.tmdb_id AND si.library_title=mi.library_title AND si.edition=mi.edition) AS max_last_downloaded
 FROM MediaItems mi) `

// GetAllSavedSets emits native JSON arrays and booleans and sorts timestamps as instants.
func (p *PostgresDB) GetAllSavedSets(ctx context.Context, filter models.DBFilter) (PagedSavedItems, logging.LogErrorInfo) {
	return postgresGetAllSavedSets(ctx, p.conn, filter)
}

// postgresSavedReader lets import validation use the same decoder before its transaction commits.
type postgresSavedReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func postgresGetAllSavedSets(ctx context.Context, conn postgresSavedReader, filter models.DBFilter) (PagedSavedItems, logging.LogErrorInfo) {
	out := PagedSavedItems{Items: []models.DBSavedItem{}}
	where, args := postgresSavedWhere(filter)
	if err := conn.QueryRowContext(ctx, postgresSavedBase+"SELECT count(*) FROM base mi "+where, args...).Scan(&out.Total); err != nil {
		return out, postgresError("Failed to count saved items", err)
	}
	sortColumn := "mi.title"
	switch strings.ToLower(strings.TrimSpace(filter.SortOption)) {
	case "year":
		sortColumn = "mi.year"
	case "library":
		sortColumn = "mi.library_title"
	case "last_downloaded", "date_downloaded":
		sortColumn = "mi.max_last_downloaded"
	}
	direction := "ASC"
	if strings.EqualFold(filter.SortOrder, "desc") {
		direction = "DESC"
	}
	query := postgresSavedBase + "SELECT " + postgresMediaJSON + "," + postgresSetsJSON + " FROM base mi " + where + " ORDER BY " + sortColumn + " " + direction + ",mi.tmdb_id,mi.library_title,mi.edition"
	pageSize := filter.ItemsPerPage
	if pageSize >= 0 {
		if pageSize == 0 {
			pageSize = 25
		}
		if pageSize > 250 {
			pageSize = 250
		}
		page := filter.PageNumber
		if page < 1 {
			page = 1
		}
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, pageSize, (page-1)*pageSize)
	}
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return out, postgresError("Failed to query saved items", err)
	}
	defer rows.Close()
	for rows.Next() {
		var mediaJSON, setsJSON []byte
		if err = rows.Scan(&mediaJSON, &setsJSON); err != nil {
			return out, postgresError("Failed to scan saved item", err)
		}
		var item models.DBSavedItem
		if err = json.Unmarshal(mediaJSON, &item.MediaItem); err != nil {
			return out, postgresError("Failed to decode saved media item", err)
		}
		if err = json.Unmarshal(setsJSON, &item.PosterSets); err != nil {
			return out, postgresError("Failed to decode saved poster sets", err)
		}
		out.Items = append(out.Items, item)
	}
	return out, postgresError("Failed to read saved items", rows.Err())
}

// postgresSavedWhere binds every filter value; identifiers and operators remain fork-owned SQL.
func postgresSavedWhere(filter models.DBFilter) (string, []any) {
	clauses := []string{"mi.set_count>0"}
	args := []any{}
	bind := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if value := strings.TrimSpace(filter.ItemTMDB_ID); value != "" {
		clauses = append(clauses, "mi.tmdb_id="+bind(value))
	}
	if strings.TrimSpace(filter.ItemLibraryTitle) != "" {
		clauses = append(clauses, "mi.library_title="+bind(filter.ItemLibraryTitle))
	}
	if filter.ItemYear > 0 {
		clauses = append(clauses, "mi.year="+bind(filter.ItemYear))
	}
	if value := strings.TrimSpace(filter.ItemTitle); value != "" {
		clauses = append(clauses, "mi.title ILIKE "+bind("%"+value+"%"))
	}
	if len(filter.LibraryTitles) > 0 {
		clauses = append(clauses, "mi.library_title=ANY("+bind(filter.LibraryTitles)+"::text[])")
	}
	setClauses := []string{}
	if filter.SetID != "" {
		setClauses = append(setClauses, "ps.set_id="+bind(filter.SetID))
	}
	if len(filter.Usernames) > 0 {
		users := []string{}
		for _, user := range filter.Usernames {
			if value := strings.TrimSpace(user); value != "" {
				users = append(users, value)
			}
		}
		setClauses = append(setClauses, "ps.\"user\"=ANY("+bind(users)+"::text[])")
	}
	switch strings.ToLower(strings.TrimSpace(filter.Autodownload)) {
	case "true", "1", "yes", "on":
		setClauses = append(setClauses, "si.autodownload=1")
	case "false", "0", "no", "off":
		setClauses = append(setClauses, "si.autodownload=0")
	}
	if len(filter.ImageTypes) > 0 {
		types := []string{}
		for _, imageType := range filter.ImageTypes {
			if value := strings.TrimSpace(imageType); value != "" {
				types = append(types, value)
			}
		}
		setClauses = append(setClauses, "EXISTS (SELECT 1 FROM ImageFiles im WHERE im.poster_set_id=ps.id AND im.item_tmdb_id=mi.tmdb_id AND im.image_type=ANY("+bind(types)+"::text[]))")
	}
	if len(setClauses) > 0 {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM SavedItems si JOIN PosterSets ps ON ps.id=si.poster_set_id WHERE si.tmdb_id=mi.tmdb_id AND si.library_title=mi.library_title AND si.edition=mi.edition AND "+strings.Join(setClauses, " AND ")+")")
	}
	if filter.MultiSetOnly {
		clauses = append(clauses, "mi.set_count>1")
	}
	switch strings.ToLower(filter.MediaItemOnServer) {
	case "true", "1", "yes", "on":
		clauses = append(clauses, "mi.on_server=1")
	case "false", "0", "no", "off":
		clauses = append(clauses, "mi.on_server=0")
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

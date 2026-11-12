# Automatically ignore items without sets

Approved scope: existing and newly discovered movies and series in the main media-server library cache.

- Add `UserPreferences.AutoIgnoreItemsWithoutSets` (`auto_ignore_items_without_sets` in JSON), default false, exposed in Settings → User Preferences.
- Reuse `Jobs.HandleTempIgnoredItems`, its existing enable switch and configurable cron schedule (default every six hours). Link the preference help to Jobs; no second schedule.
- After checking existing temporary ignores, inspect non-ignored library movies/series with TMDB IDs. Successful MediUX lookup finding zero sets applies `until-set-available`. Movie collection sets count as available.
- Lookup/database errors must not create ignores. Existing manual ignore modes must never be overwritten, including concurrent writes. Unsupported item types and missing TMDB IDs are skipped.
- Existing unignore and notification behavior stays unchanged. Disabling the preference stops new ignores, without clearing existing ignores. Disabling the job stops both scans.
- Keep existing preference persistence/import behavior; no schema migration or dependency needed.

Verification: regression tests for disabled preference, movies/shows with zero sets, movie collection sets, available sets, lookup errors, missing IDs, unsupported types, existing/manual ignores, concurrent conflict-safe persistence, and repeated scans. Run backend tests/vet and frontend lint/typecheck/format checks. Live Plex/Emby/Jellyfin/MediUX integration is outside automated verification.

Trade-off: scanning library items adds MediUX requests; schedule controls scan frequency. No separate retry policy, auto-download behavior, or per-library overrides.

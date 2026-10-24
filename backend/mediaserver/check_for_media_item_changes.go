package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"aura/notification"
	"aura/utils"
	"context"
)

func CheckForMediaItemChanges(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Checking for Media Item changes", logging.LevelDebug)
	defer logAction.Complete()

	// Get all MediaItems from the database
	dbMediaItems, logErr := database.GetAllMediaItemsWithFlags(ctx)
	if logErr.Message != "" {
		logAction.SetError("Failed to retrieve MediaItems from database", "", map[string]any{"error": logErr.Message})
		return *logAction.Error
	}

	// Compare each DB MediaItem's RatingKey with the Cache's RatingKey
	// If different, update the DB MediaItem's RatingKey
	// A DB MediaItem without Saved Sets that is not Ignored is removed by the library load
	// If a DB MediaItem is not found in the Cache AND it has Saved Sets, this is an item we want to keep in the DB but we should send a notification about it so the user can investigate
	// If a DB MediaItem is not found in the Cache AND it is Temp Ignored, this is an item we can remove from the DB
	for _, dbItem := range dbMediaItems {
		// The library load keeps the library's other items up to date
		if !dbItem.HasSavedSet && !dbItem.IsIgnored {
			continue
		}

		cachedItem, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(dbItem.LibraryTitle, dbItem.TMDB_ID, dbItem.Edition)
		if !found {
			// The exact (tmdb_id, edition) pair wasn't found
			// Check if its there under a different Edition
			// If so, this is an edition change, not a removal, reconcile it.
			if editionChangedItem, editionChangedFound := cache.LibraryStore.GetMediaItemFromSectionByTMDBID(dbItem.LibraryTitle, dbItem.TMDB_ID); editionChangedFound && editionChangedItem.Edition != dbItem.Edition {
				logging.LOGGER.Info().Timestamp().Str("tmdb_id", dbItem.TMDB_ID).Str("library_title", dbItem.LibraryTitle).
					Str("old_edition", dbItem.Edition).Str("new_edition", editionChangedItem.Edition).
					Msg("MediaItem Edition changed on media server - reconciling instead of treating as removed")

				reconcileErr := database.ReconcileMediaItemEdition(ctx, dbItem.TMDB_ID, dbItem.LibraryTitle, dbItem.Edition, *editionChangedItem)
				if reconcileErr.Message != "" {
					logAction.AppendWarning("reconcile_edition_error", reconcileErr.Message)
				} else {
					logAction.AppendResult("reconciled_editions", map[string]any{
						"tmdb_id":       dbItem.TMDB_ID,
						"library_title": dbItem.LibraryTitle,
						"old_edition":   dbItem.Edition,
						"new_edition":   editionChangedItem.Edition,
					})
				}
				continue
			}

			mediaItem := models.MediaItem{
				TMDB_ID:      dbItem.TMDB_ID,
				LibraryTitle: dbItem.LibraryTitle,
				Edition:      dbItem.Edition,
				Title:        dbItem.Title,
				Year:         dbItem.Year,
			}
			reason := ""
			action := ""
			moreInfo := ""
			if dbItem.HasSavedSet {
				// If the item is not found in the cache AND it has Saved Sets
				logging.LOGGER.Warn().Timestamp().Str("tmdb_id", dbItem.TMDB_ID).Str("library_title", dbItem.LibraryTitle).Msg("MediaItem not found in cache but has Saved Sets")
				reason = "This item was not in the cache but has Saved Sets."
				action = "This item will be kept in the database for now due to the existing Saved Sets"
				moreInfo = "This may indicate that the Media Item was removed or there is an issue with the media server. Please double check if this item exists. If it doesn't remove it from the Saved Sets."
			} else if dbItem.IsIgnored {
				// If the item is not found in the cache AND it is Temp Ignored, we can just remove it from the DB
				logging.LOGGER.Warn().Timestamp().Str("tmdb_id", dbItem.TMDB_ID).Str("library_title", dbItem.LibraryTitle).Msg("MediaItem not found in cache but is Temp Ignored - removing from database")
				reason = "This item was not in the cache but is set to be ignored temporarily."
				action = "This item will be removed from the database since it is set to be ignored temporarily"
				moreInfo = "This may indicate that the Media Item was removed or there is an issue with the media server. Please double check if this item exists. If it does exist and you want to keep it as ignored temporarily, please ignore it again."
				database.DeleteMediaItemAndIgnoredStatus(ctx, dbItem.TMDB_ID, dbItem.LibraryTitle, dbItem.Edition)
			}
			logging.LOGGER.Trace().Timestamp().Str("tmdb_id", dbItem.TMDB_ID).Str("library_title", dbItem.LibraryTitle).Str("title", dbItem.Title).Msg("Checking Media Item for changes")

			// Update the Media Item on Server in the DB
			updateErr := database.UpdateMediaItemOnServer(ctx, dbItem.TMDB_ID, dbItem.LibraryTitle, dbItem.Edition, false)
			if updateErr.Message != "" {
				logAction.AppendWarning("update_on_server_error", updateErr.Message)
			}

			sendNotFoundNotification(mediaItem, reason, action, moreInfo)
			continue
		} else if dbItem.RatingKey != cachedItem.RatingKey {
			logging.LOGGER.Trace().Timestamp().Str("tmdb_id", dbItem.TMDB_ID).Str("library_title", dbItem.LibraryTitle).
				Msgf("Updating MediaItem RatingKey from %s to %s", dbItem.RatingKey, cachedItem.RatingKey)
			database.UpdateMediaItem(ctx, *cachedItem)
			logAction.AppendResult("updated_items", map[string]any{
				"tmdb_id":        dbItem.TMDB_ID,
				"library_title":  dbItem.LibraryTitle,
				"old_rating_key": dbItem.RatingKey,
				"new_rating_key": cachedItem.RatingKey,
			})
		}
	}

	return logging.LogErrorInfo{}
}

func sendNotFoundNotification(mediaItem models.MediaItem, reason string, action string, moreInfo string) {
	// If notifications are disabled, skip
	if !config.Current.Notifications.Enabled {
		logging.LOGGER.Debug().Timestamp().Msg("Notifications are disabled, skipping app start notification")
		return
	}

	// If notification providers are not configured, skip
	if len(config.Current.Notifications.Providers) == 0 {
		logging.LOGGER.Debug().Timestamp().Msg("No notification providers configured, skipping app start notification")
		return
	}

	// If check for media item changes job notification is disabled, skip
	if !config.Current.Notifications.NotificationTemplate.CheckForMediaItemChangesJob.Enabled {
		logging.LOGGER.Debug().Timestamp().Msg("Check for media item changes job notification is disabled, skipping notification")
		return
	}

	vars := utils.TemplateVars_CheckForMediaItemChangesJob(mediaItem, reason, action, moreInfo)
	title := utils.RenderTemplate(config.Current.Notifications.NotificationTemplate.CheckForMediaItemChangesJob.Title, vars)
	message := utils.RenderTemplate(config.Current.Notifications.NotificationTemplate.CheckForMediaItemChangesJob.Message, vars)
	imageURL := ""

	ctx, ld := logging.CreateLoggingContext(context.Background(), "Notification - Send Not Found Alert")
	logAction := ld.AddAction("Sending Not Found Notification", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	defer ld.Log()
	defer logAction.Complete()

	// Send a notification to all configured providers
	for _, provider := range config.Current.Notifications.Providers {
		if provider.Enabled {
			switch provider.Provider {
			case "Discord":
				notification.SendDiscordMessage(
					ctx,
					provider.Discord,
					message,
					imageURL,
					title,
				)
			case "Pushover":
				notification.SendPushoverMessage(
					ctx,
					provider.Pushover,
					message,
					imageURL,
					title,
				)
			case "Gotify":
				notification.SendGotifyMessage(
					ctx,
					provider.Gotify,
					message,
					imageURL,
					title,
				)
			case "Webhook":
				notification.SendWebhookMessage(
					ctx,
					provider.Webhook,
					message,
					imageURL,
					title,
				)
			}
		}
	}
}

package silo

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"
)

// AuraPluginID is the plugin ID of the aura Silo plugin
const AuraPluginID = "community.aura"

// fieldImagesLocked is Silo's locked field for an item's artwork, set when an image is applied by hand
const fieldImagesLocked = 10

// How long a check for the aura plugin is kept
const auraPluginCheckTTL = time.Minute

// How long to wait after the last image of an item before refreshing it
var refreshDelay = 5 * time.Second

type auraPluginCheck struct {
	active    bool
	checkedAt time.Time
}

var (
	auraPluginChecksMu sync.Mutex
	auraPluginChecks   = map[string]auraPluginCheck{}

	pendingRefreshesMu sync.Mutex
	pendingRefreshes   = map[string]*time.Timer{}
)

// Set in tests
var (
	getQueueJobs = database.GetAllDownloadQueueJobs
	now          = time.Now
)

type siloPluginInstallationsResponse struct {
	Items []struct {
		ID       string `json:"id"`
		PluginID string `json:"plugin_id"`
		Enabled  bool   `json:"enabled"`
	} `json:"items"`
}

type siloLibraryProvidersResponse struct {
	Levels []struct {
		Entries []struct {
			PluginInstallationID string `json:"plugin_installation_id"`
			Enabled              bool   `json:"enabled"`
		} `json:"entries"`
	} `json:"levels"`
}

type siloLockedFieldsResponse struct {
	LockedFields []int `json:"locked_fields"`
}

type siloLockedFieldsRequest struct {
	LockedFields []int `json:"locked_fields"`
}

// auraPluginActive reports whether the aura plugin is enabled and in the library's providers
func auraPluginActive(ctx context.Context, s *Silo, libraryTitle string) bool {
	key := s.Config.URL + "|" + libraryTitle
	auraPluginChecksMu.Lock()
	check, ok := auraPluginChecks[key]
	auraPluginChecksMu.Unlock()
	if ok && now().Sub(check.checkedAt) < auraPluginCheckTTL {
		return check.active
	}

	// The config usually only has the library's title, the ID comes from Silo
	libraryID := ""
	for _, library := range s.Config.Libraries {
		if library.Title == libraryTitle {
			libraryID = library.ID
		}
	}
	active := false
	if libraryID == "" {
		id, Err := s.libraryID(ctx, libraryTitle)
		if Err.Message == "" {
			libraryID = id
		}
	}
	if libraryID != "" {
		active = checkAuraPlugin(ctx, s.Config, libraryID)
	}
	auraPluginChecksMu.Lock()
	auraPluginChecks[key] = auraPluginCheck{active: active, checkedAt: now()}
	auraPluginChecksMu.Unlock()
	return active
}

// checkAuraPlugin asks Silo for the aura plugin's installation and the library's providers
func checkAuraPlugin(ctx context.Context, msConfig config.Config_MediaServer, libraryID string) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Silo: Checking for the aura Plugin", logging.LevelDebug)
	defer logAction.Complete()

	u, err := apiURL(msConfig.URL, "admin", "plugins", "installations")
	if err != nil {
		logAction.AppendWarning("message", err.Error())
		return false
	}
	_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		logAction.AppendWarning("installations", Err.Message)
		return false
	}
	var installations siloPluginInstallationsResponse
	if Err := httpx.DecodeResponseToJSON(ctx, respBody, &installations, "Silo Plugin Installations"); Err.Message != "" {
		logAction.AppendWarning("installations", Err.Message)
		return false
	}
	installationID := ""
	for _, installation := range installations.Items {
		if installation.PluginID == AuraPluginID && installation.Enabled {
			installationID = installation.ID
		}
	}
	if installationID == "" {
		logAction.AppendResult("active", "the aura plugin isn't installed and enabled")
		return false
	}

	u, err = apiURL(msConfig.URL, "libraries", libraryID, "providers")
	if err != nil {
		logAction.AppendWarning("message", err.Error())
		return false
	}
	_, respBody, Err = makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		logAction.AppendWarning("providers", Err.Message)
		return false
	}
	var providers siloLibraryProvidersResponse
	if Err := httpx.DecodeResponseToJSON(ctx, respBody, &providers, "Silo Library Providers"); Err.Message != "" {
		logAction.AppendWarning("providers", Err.Message)
		return false
	}
	for _, level := range providers.Levels {
		for _, entry := range level.Entries {
			if entry.PluginInstallationID == installationID && entry.Enabled {
				logAction.AppendResult("active", true)
				return true
			}
		}
	}
	logAction.AppendResult("active", "the aura plugin isn't in the library's providers")
	return false
}

// scheduleUnlockAndRefresh unlocks and refreshes the item once its images stop coming in and it has left the Download Queue
func scheduleUnlockAndRefresh(s *Silo, item models.MediaItem) {
	pendingRefreshesMu.Lock()
	defer pendingRefreshesMu.Unlock()

	if timer, ok := pendingRefreshes[item.RatingKey]; ok {
		timer.Reset(refreshDelay)
		return
	}
	pendingRefreshes[item.RatingKey] = time.AfterFunc(refreshDelay, func() {
		ctx, ld := logging.CreateLoggingContext(context.Background(), "Silo aura Plugin Refresh")
		logAction := ld.AddAction(fmt.Sprintf("Unlock and Refresh %s", item.Title), logging.LevelInfo)
		ctx = logging.WithCurrentAction(ctx, logAction)
		defer ld.Log()

		// The set is saved when its job finishes, so wait until then
		if inDownloadQueue(ctx, item) {
			pendingRefreshesMu.Lock()
			if timer, ok := pendingRefreshes[item.RatingKey]; ok {
				timer.Reset(refreshDelay)
			}
			pendingRefreshesMu.Unlock()
			logAction.AppendResult("waiting", "the item is still in the Download Queue")
			return
		}
		pendingRefreshesMu.Lock()
		delete(pendingRefreshes, item.RatingKey)
		pendingRefreshesMu.Unlock()

		if Err := UnlockImages(ctx, s.Config, item.RatingKey); Err.Message != "" {
			logAction.SetErrorFromInfo(Err)
			return
		}
		if Err := s.RefreshMediaItemMetadata(ctx, &item, item.RatingKey, false); Err.Message != "" {
			logAction.SetErrorFromInfo(Err)
		}
	})
}

// inDownloadQueue reports whether the item has a job waiting or running
func inDownloadQueue(ctx context.Context, item models.MediaItem) bool {
	jobs, Err := getQueueJobs(ctx)
	if Err.Message != "" {
		return false
	}
	for _, job := range jobs {
		if (job.Status == "pending" || job.Status == "processing") && job.TMDB_ID == item.TMDB_ID {
			return true
		}
	}
	return false
}

// UnlockImages removes Silo's images lock from an item and keeps its other locks
func UnlockImages(ctx context.Context, msConfig config.Config_MediaServer, contentID string) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Silo: Unlocking Images for %s", contentID), logging.LevelDebug)
	defer logAction.Complete()

	// Reading an item's locks needs a profile
	if msConfig.UserID == "" {
		userID, Err := (&Silo{}).GetAdminUser(ctx, msConfig)
		if Err.Message != "" {
			logAction.SetErrorFromInfo(Err)
			return *logAction.Error
		}
		msConfig.UserID = userID
	}

	u, err := apiURL(msConfig.URL, "catalog", "items", contentID)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}
	_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	var current siloLockedFieldsResponse
	if Err := httpx.DecodeResponseToJSON(ctx, respBody, &current, "Silo Item Locked Fields"); Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	if !slices.Contains(current.LockedFields, fieldImagesLocked) {
		logAction.AppendResult("unlocked", "the images weren't locked")
		return logging.LogErrorInfo{}
	}

	remaining := slices.DeleteFunc(slices.Clone(current.LockedFields), func(field int) bool { return field == fieldImagesLocked })
	body, _ := json.Marshal(siloLockedFieldsRequest{LockedFields: remaining})
	u, err = apiURL(msConfig.URL, "admin", "items", contentID, "metadata")
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}
	_, _, Err = makeRequest(ctx, msConfig, u.String(), "PATCH", body)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	logAction.AppendResult("locked_fields", remaining)
	return logging.LogErrorInfo{}
}

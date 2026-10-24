package routes_config

import (
	"aura/config"
	"aura/jobs"
	"aura/logging"
	"aura/utils/httpx"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
)

type updateConfigSectionResponse struct {
	Message string          `json:"message"`
	Changed bool            `json:"changed"` // Whether any setting changed
	Status  AppConfigStatus `json:"status"`
}

// configSectionCheck is the result of checking an updated config section
type configSectionCheck struct {
	changed bool
	valid   bool
	applied configChanges
}

type configSection struct {
	// The section in the config, which the changed fields are decoded into
	field func(cfg *config.Config) any
	// The only fields that can be changed, if the section is more than one config field
	fields []string
	// Compare the current and new section, and validate the new section
	check func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck
}

// Config sections that can be updated, by their JSON name
var configSections = map[string]configSection{
	"auth": {
		field: func(cfg *config.Config) any { return &cfg.Auth },
		check: func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
			changed, valid := checkConfigDifferences_Auth(ctx, current.Auth, &newConfig.Auth)
			return configSectionCheck{changed: changed, valid: valid, applied: configChanges{authChanged: changed}}
		},
	},
	"logging": {
		field: func(cfg *config.Config) any { return &cfg.Logging },
		check: func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
			changed, valid := checkConfigDifferences_Logging(ctx, current.Logging, &newConfig.Logging)
			return configSectionCheck{changed: changed, valid: valid}
		},
	},
	"media_server": {
		field: func(cfg *config.Config) any { return &cfg.MediaServer },
		check: func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
			changed, valid, serverName := checkConfigDifferences_MediaServer(ctx, current.MediaServer, &newConfig.MediaServer)
			return configSectionCheck{changed: changed, valid: valid, applied: configChanges{mediaServerChanged: changed, mediaServerChecked: true, mediaServerName: serverName}}
		},
	},
	"media_servers": {
		field:  func(cfg *config.Config) any { return cfg },
		fields: []string{"media_servers", "main_media_server"},
		check:  checkMediaServersSection,
	},
	"mediux": {
		field: func(cfg *config.Config) any { return &cfg.Mediux },
		check: func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
			changed, valid := checkConfigDifferences_Mediux(ctx, current.Mediux, &newConfig.Mediux)
			return configSectionCheck{changed: changed, valid: valid, applied: configChanges{mediuxChecked: true}}
		},
	},
	"jobs": {
		field: func(cfg *config.Config) any { return &cfg.Jobs },
		check: func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
			changed, changedKeys, valid := checkConfigDifferences_Jobs(ctx, current.Jobs, &newConfig.Jobs)
			return configSectionCheck{changed: changed, valid: valid, applied: configChanges{changedJobs: changedKeys}}
		},
	},
	"backups": {
		field: func(cfg *config.Config) any { return &cfg.Backups },
		check: func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
			changed, valid := checkConfigDifferences_Backups(ctx, current.Backups, &newConfig.Backups)
			return configSectionCheck{changed: changed, valid: valid}
		},
	},
	"images": {
		field: func(cfg *config.Config) any { return &cfg.Images },
		check: func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
			changed, valid := checkConfigDifferences_Images(ctx, current.Images, &newConfig.Images, newConfig.MediaServer)
			return configSectionCheck{changed: changed, valid: valid}
		},
	},
	"tmdb": {
		field: func(cfg *config.Config) any { return &cfg.TMDB },
		check: func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
			changed, valid := checkConfigDifferences_TMDB(ctx, current.TMDB, &newConfig.TMDB)
			return configSectionCheck{changed: changed, valid: valid}
		},
	},
	"labels_and_tags": {
		field: func(cfg *config.Config) any { return &cfg.LabelsAndTags },
		check: func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
			changed, valid := checkConfigDifferences_LabelsAndTags(ctx, current.LabelsAndTags, &newConfig.LabelsAndTags)
			return configSectionCheck{changed: changed, valid: valid}
		},
	},
	"notifications": {
		field: func(cfg *config.Config) any { return &cfg.Notifications },
		check: func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
			changed, valid := checkConfigDifferences_Notifications(ctx, current.Notifications, &newConfig.Notifications)
			return configSectionCheck{changed: changed, valid: valid}
		},
	},
	"sonarr_radarr": {
		field: func(cfg *config.Config) any { return &cfg.SonarrRadarr },
		check: func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
			changed, valid := checkConfigDifferences_SonarrRadarr(ctx, current.SonarrRadarr, &newConfig.SonarrRadarr, newConfig.MediaServer)
			return configSectionCheck{changed: changed, valid: valid}
		},
	},
	"user_preferences": {
		field: func(cfg *config.Config) any { return &cfg.UserPreferences },
		check: func(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
			changed, valid := checkConfigDifferences_UserPreferences(ctx, current.UserPreferences, &newConfig.UserPreferences)
			return configSectionCheck{changed: changed, valid: valid}
		},
	},
}

// UpdateConfigSection godoc
// @Summary      Update Config Section
// @Description  Update some settings in one section of the application configuration. Only the settings included in the request body are changed. The section is validated and saved on its own, and only that section's changes are applied (e.g. reconnecting to the media server, restarting changed jobs).
// @Tags         Config
// @Accept       json
// @Produce      json
// @Param        section  path      string  true  "Config section"  Enums(auth, logging, media_server, media_servers, mediux, jobs, backups, images, tmdb, labels_and_tags, notifications, sonarr_radarr, user_preferences)
// @Param        changes  body      object  true  "The settings to change, using the same field names as the config (e.g. {\"url\": \"http://plex:32400\"} for media_server)"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200        {object}  httpx.JSONResponse{data=routes_config.updateConfigSectionResponse}
// @Failure      500        {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/config/{section} [patch]
func UpdateAppConfigSection(w http.ResponseWriter, r *http.Request) {
	sectionName := chi.URLParam(r, "section")
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction(fmt.Sprintf("Update Config Section: %s", sectionName), logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var response updateConfigSectionResponse

	section, found := configSections[sectionName]
	if !found {
		validSections := make([]string, 0, len(configSections))
		for name := range configSections {
			validSections = append(validSections, name)
		}
		sort.Strings(validSections)
		logAction.SetError(fmt.Sprintf("Unknown config section '%s'", sectionName), fmt.Sprintf("Valid sections are: %s", strings.Join(validSections, ", ")), nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	// Only one config update at a time, so updates can't overwrite each other
	config.UpdateLock.Lock()
	defer config.UpdateLock.Unlock()

	// Apply the changed fields to a copy of the current config
	current := config.Current
	newConfig := current.Clone()
	Err := decodeConfigSectionChanges(ctx, r.Body, section.field(&newConfig), section.fields, sectionName)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		httpx.SendResponse(w, ld, response)
		return
	}

	// Check and validate the section
	result := section.check(ctx, current, &newConfig)
	if !result.valid {
		Err := firstSubActionError(logAction)
		if Err.Message == "" {
			Err = logging.LogErrorInfo{Message: fmt.Sprintf("Invalid %s settings", sectionName), Help: "Check the logs for details."}
		}
		Err.Detail = withDetail(Err.Detail, "section", sectionName)
		logAction.SetErrorFromInfo(Err)
		response.Message = Err.Message
		httpx.SendResponse(w, ld, response)
		return
	}

	if !result.changed {
		response.Message = "No changes detected in configuration"
		response.Status = currentConfigStatus(ctx)
		httpx.SendResponse(w, ld, response)
		return
	}

	// If the config was invalid before, check it again
	if !config.Valid {
		newConfig.Validate(ctx)
	}

	// Save the new config. Only the changed values are written
	saveErr := newConfig.Save(ctx)
	if saveErr.Message != "" {
		logAction.SetErrorFromInfo(saveErr)
		httpx.SendResponse(w, ld, response)
		return
	}

	// Update the global config variable
	config.Current = newConfig
	config.Loaded = true
	if result.applied.mediaServerChecked {
		config.MediaServerValid = true
		config.MediaServerName = result.applied.mediaServerName
	}
	if result.applied.mediuxChecked {
		config.MediuxValid = true
	}

	applyConfigChanges(ctx, logAction, result.applied)

	response.Message = "Configuration saved"
	response.Changed = true
	response.Status = currentConfigStatus(ctx)
	httpx.SendResponse(w, ld, response)
}

// decodeConfigSectionChanges applies the changed fields from the request body to the section.
// Unknown fields are rejected.
func decodeConfigSectionChanges(ctx context.Context, body io.ReadCloser, target any, fields []string, sectionName string) logging.LogErrorInfo {
	_, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Decoding changes for config section: %s", sectionName), logging.LevelTrace)
	defer logAction.Complete()
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		logAction.SetError("Failed to read request body", "Ensure the request body can be read", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	var changes map[string]json.RawMessage
	if err := json.Unmarshal(data, &changes); err != nil || changes == nil {
		logAction.SetError("Invalid request body", "The request body must be a JSON object with the settings to change", nil)
		return *logAction.Error
	}

	if fields != nil {
		for name := range changes {
			if !slices.Contains(fields, name) {
				logAction.SetError(fmt.Sprintf("Invalid changes for config section '%s'", sectionName),
					fmt.Sprintf("Only %s can be changed in this section", strings.Join(fields, " and ")), map[string]any{"error": fmt.Sprintf("unknown field '%s'", name)})
				return *logAction.Error
			}
		}
	}

	if err := mergeConfigChanges(reflect.ValueOf(target).Elem(), changes, ""); err != nil {
		logAction.SetError(fmt.Sprintf("Invalid changes for config section '%s'", sectionName), "Use the same field names as the config, e.g. {\"url\": \"http://plex:32400\"}", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	return logging.LogErrorInfo{}
}

// mergeConfigChanges sets the changed fields on target.
// Objects are merged field by field; lists, maps and values are replaced.
func mergeConfigChanges(target reflect.Value, changes map[string]json.RawMessage, path string) error {
	fields := jsonFieldIndexes(target.Type())
	for name, raw := range changes {
		index, found := fields[name]
		if !found {
			return fmt.Errorf("unknown field '%s%s'", path, name)
		}
		field := target.Field(index)

		if field.Kind() == reflect.Struct {
			var nested map[string]json.RawMessage
			if err := json.Unmarshal(raw, &nested); err != nil || nested == nil {
				return fmt.Errorf("'%s%s' must be an object", path, name)
			}
			if err := mergeConfigChanges(field, nested, path+name+"."); err != nil {
				return err
			}
			continue
		}

		// Decode into a new value, so nothing is kept from the old one
		value := reflect.New(field.Type())
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(value.Interface()); err != nil {
			return fmt.Errorf("'%s%s': %w", path, name, err)
		}
		field.Set(value.Elem())
	}
	return nil
}

// jsonFieldIndexes returns the index of each struct field by its JSON name, skipping json:"-" fields
func jsonFieldIndexes(t reflect.Type) map[string]int {
	indexes := map[string]int{}
	for i := 0; i < t.NumField(); i++ {
		structField := t.Field(i)
		name, _, _ := strings.Cut(structField.Tag.Get("json"), ",")
		if !structField.IsExported() || name == "-" {
			continue
		}
		if name == "" {
			name = structField.Name
		}
		indexes[name] = i
	}
	return indexes
}

// firstSubActionError returns the first error from the sub-actions
func firstSubActionError(action *logging.LogAction) logging.LogErrorInfo {
	for _, subAction := range action.SubActions {
		if subAction.Error != nil && subAction.Error.Message != "" {
			return *subAction.Error
		}
		if Err := firstSubActionError(subAction); Err.Message != "" {
			return Err
		}
	}
	return logging.LogErrorInfo{}
}

func withDetail(detail map[string]any, key string, value any) map[string]any {
	if detail == nil {
		detail = map[string]any{}
	}
	detail[key] = value
	return detail
}

// checkMediaServersSection checks the media_servers section: the media servers that aren't the main one.
// The main media server is changed in the media_server section.
func checkMediaServersSection(ctx context.Context, current config.Config, newConfig *config.Config) configSectionCheck {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Section: media_servers", logging.LevelTrace)
	defer logAction.Complete()

	// Changing the main media server moves the saved items to it, which is done with Make Main
	if current.MainMediaServer != "" && newConfig.MainMediaServer != current.MainMediaServer {
		logAction.SetError("The main media server is changed with Make Main", "Use Make Main on the media server in Settings, or POST /api/config/main-media-server", nil)
		return configSectionCheck{valid: false}
	}
	// The main media server can't be removed
	if current.MainMediaServer != "" && !slices.ContainsFunc(newConfig.MediaServers, func(server config.Config_MediaServer) bool {
		return server.ID == current.MainMediaServer
	}) {
		logAction.SetError("The main media server can't be removed", "Keep the main media server in media_servers", nil)
		return configSectionCheck{valid: false}
	}
	// The first media server becomes the main one
	if newConfig.MainMediaServer == "" && len(newConfig.MediaServers) > 0 {
		if newConfig.MediaServers[0].ID == "" {
			newConfig.MediaServers[0].ID = config.NewMediaServerID()
		}
		newConfig.MainMediaServer = newConfig.MediaServers[0].ID
	}

	// The main media server is changed in the media_server section, so its current settings are kept
	for i := range newConfig.MediaServers {
		if current.MainMediaServer != "" && newConfig.MediaServers[i].ID == current.MainMediaServer {
			newConfig.MediaServers[i] = current.MediaServer
		}
	}

	// Check the main media server the same way as the media_server section, if it is new
	config.RestoreMediaServerTokens(current.MediaServers, newConfig.MediaServers)
	newConfig.LoadMainMediaServer()
	newConfig.MediaServer.UserID = current.MediaServer.UserID
	mainChanged, mainChecked, serverName := false, false, ""
	if !reflect.DeepEqual(storedMediaServers([]config.Config_MediaServer{current.MediaServer}), storedMediaServers([]config.Config_MediaServer{newConfig.MediaServer})) {
		valid := false
		mainChanged, valid, serverName = checkConfigDifferences_MediaServer(ctx, current.MediaServer, &newConfig.MediaServer)
		if !valid {
			return configSectionCheck{valid: false}
		}
		mainChecked = true
	}

	changed, valid := checkConfigDifferences_MediaServers(ctx, current, newConfig)
	return configSectionCheck{
		changed: mainChanged || changed,
		valid:   valid,
		applied: configChanges{mediaServerChanged: mainChanged, mediaServersChanged: changed, mediaServerChecked: mainChecked, mediaServerName: serverName},
	}
}

// checkConfigDifferences_UserPreferences compares old and new UserPreferences configurations.
func checkConfigDifferences_UserPreferences(ctx context.Context, oldPreferences config.Config_UserPreferences, newPreferences *config.Config_UserPreferences) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: UserPreferences", logging.LevelTrace)
	defer logAction.Complete()

	newValid = config.ValidateUserPreferences(ctx, newPreferences)
	changed = !reflect.DeepEqual(oldPreferences, *newPreferences)
	if changed {
		logAction.AppendResult("UserPreferences changed", fmt.Sprintf("from %+v to %+v", oldPreferences, *newPreferences))
	}
	return changed, newValid
}

// configChanges are the changes to apply after the config is saved
type configChanges struct {
	authChanged         bool
	mediaServerChanged  bool
	mediaServersChanged bool   // Whether the media servers that aren't the main one changed
	mediaServerChecked  bool   // Whether the media server connection was checked
	mediaServerName     string // Name of the media server, if it was checked
	mediuxChecked       bool   // Whether the MediUX token was checked
	changedJobs         []jobs.JobKey
}

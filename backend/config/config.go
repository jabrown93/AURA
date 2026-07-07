package config

import "aura/models"

var (
	// Config State Variables
	Current    Config
	ConfigPath string = ""

	// Flags to track config loading and validation status
	Loaded           bool   = false
	Valid            bool   = false
	MediaServerValid bool   = true
	MediaServerName  string = ""
	MediuxValid      bool   = true
	AppFullyLoaded   bool   = false
	AppLoadingStep   string = ""

	// App Details
	AppName    string = ""
	AppAuthor  string = ""
	AppLicense string = ""
	AppPort    int    = 0
	AppVersion string = ""
)

type Config struct {
	Auth          Config_Auth              `json:"auth" yaml:"Auth,omitempty"`                     // Authentication settings.
	Logging       Config_Logging           `json:"logging" yaml:"Logging,omitempty"`               // Logging configuration settings.
	MediaServer   Config_MediaServer       `json:"media_server" yaml:"MediaServer,omitempty"`      // Media server integration settings.
	Mediux        Config_Mediux            `json:"mediux" yaml:"Mediux,omitempty"`                 // MediUX integration settings.
	AutoDownload  Config_AutoDownload      `json:"auto_download" yaml:"AutoDownload,omitempty"`    // Auto-download settings.
	Images        Config_Images            `json:"images" yaml:"Images,omitempty"`                 // Image settings.
	TMDB          Config_TMDB              `json:"tmdb" yaml:"TMDB,omitempty"`                     // TMDB (The Movie Database) integration settings.
	LabelsAndTags Config_LabelsAndTags     `json:"labels_and_tags" yaml:"LabelsAndTags,omitempty"` // Labels and tags settings.
	Notifications Config_Notifications     `json:"notifications" yaml:"Notifications,omitempty"`   // Notification settings.
	SonarrRadarr  Config_SonarrRadarr_Apps `json:"sonarr_radarr" yaml:"SonarrRadarr,omitempty"`    // List of Sonarr/Radarr instances to integrate with.
	Database      Config_Database          `json:"database" yaml:"Database,omitempty"`             // Database configuration settings.
}

type Config_Dev struct {
	Enabled   bool   `json:"enabled" yaml:"Enabled,omitempty"`      // Whether to enable development mode.
	LocalPath string `json:"local_path" yaml:"LocalPath,omitempty"` // Local path for development mode.
}
type Config_Auth struct {
	Enabled  bool   `json:"enabled" yaml:"Enabled"`             // Whether to enable authentication.
	Password string `json:"password" yaml:"Password,omitempty"` // Password for authentication.
}

type Config_Logging struct {
	Level string `json:"level" yaml:"Level"`         // Logging level (e.g., TRACE, DEBUG, INFO, WARN, ERROR).
	File  string `json:"file" yaml:"File,omitempty"` // File path for logging output.
}

type Config_MediaServer struct {
	Type                         string                  `json:"type" yaml:"Type"`                                                      // Type of media server (e.g., plex, emby, jellyfin).
	URL                          string                  `json:"url" yaml:"URL"`                                                        // Base URL of the media server. This is either the IP:Port or the domain name (e.g., plex.domain.com).
	ApiToken                     string                  `json:"api_token" yaml:"ApiToken"`                                             // Authentication token for accessing the media server.
	Libraries                    []models.LibrarySection `json:"libraries,omitempty" yaml:"Libraries,omitempty"`                        // List of media server libraries to manage.
	UserID                       string                  `json:"user_id,omitempty" yaml:"UserID,omitempty"`                             // User ID for accessing the media server. This is used for Emby and Jellyfin servers.
	EnableSortByEpisodeAddedDate bool                    `json:"enable_sort_by_episode_added_date" yaml:"EnableSortByEpisodeAddedDate"` // Whether to check episodes for added date when getting Media Items. This is only for Plex servers.
	EnablePlexEventListener      bool                    `json:"enable_plex_event_listener" yaml:"EnablePlexEventListener"`             // Whether to enable the Plex event listener for reapplying images on refresh. Plex exclusive feature.
}

type Config_Mediux struct {
	ApiToken        string `json:"api_token" yaml:"ApiToken"`               // Authentication token for accessing MediUX services.
	DownloadQuality string `json:"download_quality" yaml:"DownloadQuality"` // Quality of the media to download from MediUX (Options: "original", "optimized") Defaults to "optimized".
}

type Config_AutoDownload struct {
	Enabled bool   `json:"enabled" yaml:"Enabled"`               // Whether auto-download is enabled.
	Cron    string `json:"cron,omitempty" yaml:"Cron,omitempty"` // Cron expression for scheduling auto-downloads.
}

type Config_Images struct {
	CacheImages       Config_CacheImages       `json:"cache_images" yaml:"CacheImages"`              // Settings for caching images.
	SaveImagesLocally Config_SaveImagesLocally `json:"save_images_locally" yaml:"SaveImagesLocally"` // Settings for saving images locally alongside content.
	Kometa            Config_Kometa            `json:"kometa" yaml:"Kometa"`                         // Settings for Kometa asset directory integration (Plex only).
}

type Config_CacheImages struct {
	Enabled bool `json:"enabled" yaml:"Enabled"` // Whether to enable caching of images.
}

type Config_SaveImagesLocally struct {
	Enabled                 bool   `json:"enabled" yaml:"Enabled"`                                                       // Whether to save images next to their content.
	Path                    string `json:"path,omitempty" yaml:"Path,omitempty"`                                         // By default, this is set to alongside the content. If set, this will override that behavior and save all images to this path.
	EpisodeNamingConvention string `json:"episode_naming_convention,omitempty" yaml:"EpisodeNamingConvention,omitempty"` // Episode naming convention for the media server. Only needed for Plex. Will default to match
	RunningOnWindows        bool   `json:"running_on_windows,omitempty" yaml:"RunningOnWindows,omitempty"`               // Whether the application is running on Windows. This affects path formatting.
}

type Config_Kometa struct {
	Enabled              bool   `json:"enabled" yaml:"Enabled"`                                                 // Whether to write downloaded images into the Kometa asset directory using Kometa naming conventions. Plex exclusive feature.
	AssetDirectory       string `json:"asset_directory,omitempty" yaml:"AssetDirectory,omitempty"`              // Path to the Kometa asset directory (the same directory Kometa reads assets from). Uses the folder-per-item (asset_folders: true) layout.
	ImportCron           string `json:"import_cron,omitempty" yaml:"ImportCron,omitempty"`                      // Optional cron expression for periodically importing existing Kometa assets. Empty means import is manual only.
	SonarrRadarrFallback bool   `json:"sonarr_radarr_fallback,omitempty" yaml:"SonarrRadarrFallback,omitempty"` // When a media-server lookup fails (e.g. Plex returns a 404) but the item exists in Sonarr/Radarr, still write the downloaded images into the Kometa asset directory, deriving the asset folder name from the Sonarr/Radarr path. Requires Enabled. Plex exclusive feature.
}

type Config_TMDB struct {
	ApiToken string `json:"-" yaml:"ApiToken"` // API token for accessing TMDB (The Movie Database) services.
}

type Config_LabelsAndTags struct {
	Applications                           []Config_LabelsAndTagsProvider `json:"applications,omitempty" yaml:"Applications,omitempty"`
	RemoveOverlayLabelOnlyOnPosterDownload bool                           `json:"remove_overlay_label_only_on_poster_download,omitempty" yaml:"RemoveOverlayLabelOnlyOnPosterDownload,omitempty"` // Whether to remove the "Overlay" label from media items after downloading a poster image. This is to allow Kometa to reprocess the image and apply the overlays. This is a Plex exclusive feature and should be set to true if you have "Overlay" in your "Remove" list for the Plex application under "LabelsAndTags".
}

type Config_LabelsAndTagsProvider struct {
	Application                 string   `json:"application,omitempty" yaml:"Application,omitempty"`
	Enabled                     bool     `json:"enabled,omitempty" yaml:"Enabled,omitempty"`
	Add                         []string `json:"add,omitempty" yaml:"Add,omitempty"`
	Remove                      []string `json:"remove,omitempty" yaml:"Remove,omitempty"`
	AddLabelTagForSelectedTypes bool     `json:"add_label_tag_for_selected_types,omitempty" yaml:"AddLabelTagForSelectedTypes,omitempty"`
}

type Config_Notifications struct {
	Enabled              bool                           `json:"enabled" yaml:"Enabled"`                                    // Whether this notification method is enabled
	Providers            []Config_Notification_Provider `json:"providers,omitempty" yaml:"Providers,omitempty"`            // List of notification providers
	NotificationTemplate Config_NotificationTemplate    `json:"templates,omitempty" yaml:"NotificationTemplate,omitempty"` // Custom notification templates for different events
}

type Config_Notification_Provider struct {
	Provider string                        `json:"provider,omitempty" yaml:"Provider,omitempty"` // Notification provider
	Enabled  bool                          `json:"enabled,omitempty" yaml:"Enabled,omitempty"`   // Whether this notification method is enabled
	Discord  *Config_Notification_Discord  `json:"discord,omitempty" yaml:"Discord,omitempty"`   // Discord notification settings
	Pushover *Config_Notification_Pushover `json:"pushover,omitempty" yaml:"Pushover,omitempty"` // Pushover notification settings
	Gotify   *Config_Notification_Gotify   `json:"gotify,omitempty" yaml:"Gotify,omitempty"`     // Gotify notification settings
	Webhook  *Config_Notification_Webhook  `json:"webhook,omitempty" yaml:"Webhook,omitempty"`   // Webhook notification settings
}

type Config_Notification_Discord struct {
	Webhook string `json:"webhook,omitempty" yaml:"Webhook,omitempty"` // Webhook URL for the Discord notification provider.
}

type Config_Notification_Pushover struct {
	ApiToken string `json:"api_token,omitempty" yaml:"ApiToken,omitempty"` // Token for the Pushover notification provider.
	UserKey  string `json:"user_key,omitempty" yaml:"UserKey,omitempty"`   // UserKey for the Pushover notification provider.
}

type Config_Notification_Gotify struct {
	URL      string `json:"url,omitempty" yaml:"URL,omitempty"`            // URL for the Gotify notification provider.
	ApiToken string `json:"api_token,omitempty" yaml:"ApiToken,omitempty"` // Token for the Gotify notification provider.
}

type Config_Notification_Webhook struct {
	URL     string            `json:"url,omitempty" yaml:"URL,omitempty"`         // URL for the Webhook notification provider.
	Headers map[string]string `json:"headers,omitempty" yaml:"Headers,omitempty"` // Headers for the Webhook notification provider.
}

type Config_NotificationTemplate struct {
	// Any additional custom notification templates should be added here. You will also need to update the following files to ensure the new template is fully integrated:
	// - backend/config/defaults.go
	// - backend/config/template_variables.go
	// - backend/config/validate.go
	// - backend/routing/config/update.go
	// - backend/routing/validation/notification.go
	// - backend/utils/variable_filler.go
	// - frontend/src/components/settings-onboarding/ConfigSectionNotifications.tsx
	// - frontend/src/types/config/config-default-app.ts
	// - frontend/src/types/config/config.ts

	AppStartup                      Config_CustomNotification `json:"app_startup" yaml:"AppStartup,omitempty"`                                               // Custom notification settings for application startup.
	TestNotification                Config_CustomNotification `json:"test_notification" yaml:"TestNotification,omitempty"`                                   // Custom notification settings for test notifications.
	Autodownload                    Config_CustomNotification `json:"autodownload" yaml:"AutoDownload,omitempty"`                                            // Custom notification settings for auto-download events.
	DownloadQueue                   Config_CustomNotification `json:"download_queue" yaml:"DownloadQueue,omitempty"`                                         // Custom notification settings for download queue events.
	NewSetsAvailableForIgnoredItems Config_CustomNotification `json:"new_sets_available_for_ignored_items" yaml:"NewSetsAvailableForIgnoredItems,omitempty"` // Custom notification settings for when new sets become available for ignored items.
	CheckForMediaItemChangesJob     Config_CustomNotification `json:"check_for_media_item_changes_job" yaml:"CheckForMediaItemChangesJob,omitempty"`         // Custom notification settings for the media item changes job.
	SonarrNotification              Config_CustomNotification `json:"sonarr_notification" yaml:"SonarrNotification,omitempty"`                               // Custom notification settings for Sonarr events.
}

type Config_CustomNotification struct {
	Enabled      bool   `json:"enabled" yaml:"Enabled"`                                // Whether custom notifications are enabled.
	Title        string `json:"title,omitempty" yaml:"Title,omitempty"`                // Title for the custom notification.
	Message      string `json:"message,omitempty" yaml:"Message,omitempty"`            // Message for the custom notification.
	IncludeImage bool   `json:"include_image,omitempty" yaml:"IncludeImage,omitempty"` // Whether to include an image with the custom notification.
}

type Config_SonarrRadarr_Apps struct {
	Applications []Config_SonarrRadarrApp `json:"applications,omitempty" yaml:"Applications,omitempty"` // List of Sonarr/Radarr applications to integrate with.
}

type Config_SonarrRadarrApp struct {
	Type     string `json:"type,omitempty" yaml:"Type,omitempty"`          // Type of service (either "sonarr" or "radarr").
	Library  string `json:"library,omitempty" yaml:"Library,omitempty"`    // Name of the Media Server library associated with this Sonarr/Radarr instance.
	URL      string `json:"url,omitempty" yaml:"URL,omitempty"`            // Base URL of the Sonarr/Radarr server.
	ApiToken string `json:"api_token,omitempty" yaml:"ApiToken,omitempty"` // API key for accessing the Sonarr/Radarr server.
}

type Config_Database struct {
	Type     string `json:"type,omitempty" yaml:"Type,omitempty"`         // Type of database (e.g., "sqlite", "mysql", "postgresql").
	Path     string `json:"path,omitempty" yaml:"Path,omitempty"`         // File path for the database (if applicable, e.g., for SQLite).
	User     string `json:"user,omitempty" yaml:"User,omitempty"`         // Username for database authentication (if applicable).
	Password string `json:"password,omitempty" yaml:"Password,omitempty"` // Password for database authentication (if applicable).
	Host     string `json:"host,omitempty" yaml:"Host,omitempty"`         // Hostname or IP address of the database server (if applicable).
	Port     int    `json:"port,omitempty" yaml:"Port,omitempty"`         // Port number of the database server (if applicable).
	Name     string `json:"name,omitempty" yaml:"Name,omitempty"`         // Name of the database to connect to.
	DSN      string `json:"dsn,omitempty" yaml:"DSN,omitempty"`           // Data Source Name for the database connection (if applicable).
}

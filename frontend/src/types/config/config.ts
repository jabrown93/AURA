export interface AppConfig {
  auth: AppConfigAuth;
  logging: AppConfigLogging; // Logging configuration settings
  media_server: AppConfigMediaServer; // Media server integration settings
  mediux: AppConfigMediux; // MediUX integration settings
  auto_download: AppConfigAutoDownload; // Auto-download settings
  images: AppConfigImages;
  tmdb: AppConfigTMDB; // TMDB (The Movie Database) integration settings
  labels_and_tags: AppConfigLabelsAndTags; // Labels and tags management settings
  notifications: AppConfigNotifications; // Notification settings
  sonarr_radarr: AppConfigSonarrRadarrApps; // List of Sonarr/Radarr instances to integrate with
}

export interface AppConfigAuth {
  enabled: boolean; // Whether authentication is enabled
  password: string; // Hashed password for authentication
}

export interface AppConfigLogging {
  level: string; // Logging level (e.g., DEBUG, INFO, WARN, ERROR)
  file?: string; // Log file path
}

export interface AppConfigMediaServer {
  type: string; // Type of media server (e.g., plex, emby, jellyfin)
  url: string; // Base URL of the media server
  api_token: string; // Authentication token for accessing the media server
  libraries: AppConfigMediaServerLibrary[]; // List of media server libraries to manage
  user_id?: string; // User ID for accessing the media server (optional for Emby/Jellyfin)
  enable_sort_by_episode_added_date?: boolean; // Whether to enable sorting shows by latest episode added date (Plex only)
  enable_plex_event_listener?: boolean; // Whether to enable the Plex event listener for reapplying images on refresh (Plex only)
}

export interface AppConfigMediaServerLibrary {
  title: string; // Name of the library
  id: string; // Unique identifier for the library section
  type: string; // Type of the library (e.g., movie, show)
  path?: string; // Filesystem path to the library
}

export interface AppConfigMediux {
  api_token: string; // Authentication token for accessing MediUX services
  download_quality: string; // Preferred download quality (e.g., "original", "optimized")
}

export interface AppConfigAutoDownload {
  enabled: boolean; // Whether auto-download is enabled
  cron: string; // Cron expression for scheduling auto-downloads
}

export interface AppConfigImages {
  cache_images: AppConfigCacheImages;
  save_images_locally: AppConfigSaveImagesLocally;
  kometa: AppConfigKometa;
}

export interface AppConfigCacheImages {
  enabled: boolean; // Whether to enable caching of images.
}

export interface AppConfigSaveImagesLocally {
  enabled: boolean; // Whether to save images locally.
  path: string; // Path to save images locally. If empty, images will be saved next to content.
  episode_naming_convention: string; // Naming convention for episode images.
}

export interface AppConfigKometa {
  enabled: boolean; // Whether to write downloaded images into the Kometa asset directory (Plex only).
  asset_directory: string; // Path to the Kometa asset directory (folder-per-item layout).
  import_cron: string; // Optional cron for importing existing Kometa assets. Empty = manual only.
}

export interface AppConfigTMDB {
  api_token: string; // API key for accessing TMDB services
}

export interface AppConfigLabelsAndTags {
  applications: AppConfigLabelsAndTagsApplication[];
  remove_overlay_label_only_on_poster_download: boolean; // Whether to remove the "Overlay" label from media items after downloading a poster image. This is to allow Kometa to reprocess the image and apply the overlays. This is a Plex exclusive feature and should be set to true if you have "Overlay" in your "Remove" list for the Plex application under "LabelsAndTags".
}

export interface AppConfigLabelsAndTagsApplication {
  application: string; // Name of the application (e.g., Plex)
  enabled: boolean; // Whether label/tag management is enabled for this application
  add: string[]; // List of labels/tags to add
  remove: string[]; // List of labels/tags to remove
  add_label_tag_for_selected_types: boolean; // Whether to add labels/tags for selected media types (poster, backdrop, etc.)
}

export interface AppConfigNotifications {
  enabled: boolean;
  providers: AppConfigNotificationProviders[];
  templates: AppConfigNotificationTemplate;
}

export interface AppConfigNotificationProviders {
  provider: string;
  enabled: boolean;
  discord?: AppConfigNotificationDiscord;
  pushover?: AppConfigNotificationPushover;
  gotify?: AppConfigNotificationGotify;
  webhook?: AppConfigNotificationWebhook;
}

export interface AppConfigNotificationDiscord {
  enabled: boolean;
  webhook: string;
}

export interface AppConfigNotificationPushover {
  enabled: boolean;
  user_key: string;
  api_token: string;
}

export interface AppConfigNotificationGotify {
  enabled: boolean;
  url: string;
  api_token: string;
}

export interface AppConfigNotificationWebhook {
  enabled: boolean;
  url: string;
  headers: { [key: string]: string }; // Key-value pairs for custom headers
}

export interface AppConfigNotificationTemplate {
  app_startup: AppConfigNotificationCustomNotification;
  test_notification: AppConfigNotificationCustomNotification;
  autodownload: AppConfigNotificationCustomNotification;
  download_queue: AppConfigNotificationCustomNotification;
  new_sets_available_for_ignored_items: AppConfigNotificationCustomNotification;
  check_for_media_item_changes_job: AppConfigNotificationCustomNotification;
  sonarr_notification: AppConfigNotificationCustomNotification;
}

export interface NotificationTemplateVariablesCatalog {
  template_variables: Record<string, string[]>;
}

export interface AppConfigNotificationCustomNotification {
  enabled: boolean;
  title: string;
  message: string;
  include_image: boolean;
}

export interface AppConfigSonarrRadarrApps {
  applications: AppConfigSonarrRadarrApp[];
}
export interface AppConfigSonarrRadarrApp {
  type: string; // Type of service (either "sonarr" or "radarr").
  library: string; // Name of the Media Server library associated with this Sonarr/Radarr instance.
  url: string; // Base URL of the Sonarr/Radarr server.
  api_token: string; // API key for accessing the Sonarr/Radarr server.
}

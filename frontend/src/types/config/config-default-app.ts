import type { AppConfig } from "@/types/config/config";

// Central default
export const defaultAppConfig = (): AppConfig =>
  ({
    auth: {
      enabled: false,
      password: "",
    },
    logging: {
      level: "",
      file: "",
    },
    media_server: {
      type: "",
      url: "",
      api_token: "",
      libraries: [],
      user_id: "",
    },
    mediux: {
      api_token: "",
      download_quality: "",
    },
    auto_download: {
      enabled: false,
      cron: "",
    },
    images: {
      cache_images: { enabled: false },
      save_images_locally: {
        enabled: false,
        path: "",
        episode_naming_convention: "",
      },
      kometa: {
        enabled: false,
        asset_directory: "",
        import_cron: "",
        sonarr_radarr_fallback: false,
      },
    },
    tmdb: {
      api_token: "",
    },
    labels_and_tags: {
      applications: [],
      remove_overlay_label_only_on_poster_download: false,
    },
    notifications: {
      enabled: false,
      providers: [],
      templates: {
        app_startup: {
          enabled: true,
          title: "Startup",
          message: "The application has started.",
          include_image: false,
        },
        test_notification: {
          enabled: true,
          title: "Test Notification",
          message: "This is a test notification.",
          include_image: false,
        },
        autodownload: {
          enabled: true,
          title: "Auto Download | {{ReasonTitle}}",
          message:
            "{{MediaItemTitle}}{{NewLine}}{{ImageName}}{{NewLine}}Set ID: {{SetID}}{{NewLine}}{{NewLine}}Reason:{{NewLine}}{{Reason}}",
          include_image: true,
        },
        download_queue: {
          enabled: true,
          title: "Download Queue | {{ReasonTitle}}",
          message:
            "{{MediaItemTitle}}{{NewLine}}{{ImageName}}{{NewLine}}Set ID: {{SetID}}{{NewLine}}{{NewLine}}Reason:{{NewLine}}{{Reason}}",
          include_image: true,
        },
        new_sets_available_for_ignored_items: {
          enabled: true,
          title: "New Sets Available for Ignored Item",
          message:
            "A new set has been detected for the previously ignored item '{{MediaItemTitle}}'. It is now a part of {{SetCount}} sets in MediUX, and will no longer be ignored in {{AppName}}.",
          include_image: true,
        },
        check_for_media_item_changes_job: {
          enabled: true,
          title: "Check For Media Item Changes Job",
          message:
            "The media item '{{MediaItemTitle}}' (TMDB ID: {{MediaItemTMDBID}}) in library '{{MediaItemLibraryTitle}}' could not be found in the media server cache.{{NewLine}}Reason:{{NewLine}}{{Reason}}{{NewLine}}{{NewLine}}{{Action}}{{NewLine}}{{MoreInfo}}",
          include_image: false,
        },
        sonarr_notification: {
          enabled: true,
          title: "Sonarr Notification | {{ReasonTitle}}",
          message:
            "{{MediaItemTitle}}{{NewLine}}{{ImageName}}{{NewLine}}Set ID: {{SetID}}{{NewLine}}Reason:{{NewLine}}{{Reason}}{{NewLine}}{{Result}}",
          include_image: true,
        },
      },
    },
    sonarr_radarr: {
      applications: [{ type: "", library: "", url: "", api_token: "" }],
    },
  }) satisfies AppConfig;

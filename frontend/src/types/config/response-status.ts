import type { AppConfig } from "@/types/config/config";

export interface AppStatusResponse {
  config_loaded: boolean;
  config_valid: boolean;
  needs_setup: boolean;
  media_server_reachable: boolean;
  mediux_reachable: boolean;
  mediux_valid: boolean;
  current_setup: AppConfig;
  media_server_name?: string;
  mediux_site_link?: string;
  app_fully_loaded: boolean;
  app_version: string;
  app_loading_step: string;
  api_key_configured: boolean;
  auth_disabled_by_env?: boolean; // AURA_DISABLE_AUTH has turned off authentication
  setup_code_required?: boolean; // The setup code must be entered first
  config_has_errors?: boolean; // The required settings are entered, but have errors
}

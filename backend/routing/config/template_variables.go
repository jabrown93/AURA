package routes_config

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type templateVariablesResponse struct {
	Variables          config.NotificationTemplateVariableCatalog `json:"variables"`
	ImageNameVariables []string                                   `json:"image_name_variables"` // Variables the saved image file names can use
	ImageNameDefaults  config.Config_ImageNameTemplates           `json:"image_name_defaults"`  // The standard file names
}

// GetNotificationTemplateVariables godoc
// @Summary      Get Notification Template Variables
// @Description  Get all available notification template variables grouped by category and by template type, and the variables and standard names for saved image file names.
// @Tags         Config
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=routes_config.templateVariablesResponse}
// @Router       /api/config/template-variables [get]
func GetNotificationTemplateVariables(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Notification Template Variables", logging.LevelTrace)
	ctx = logging.WithCurrentAction(ctx, logAction)

	response := templateVariablesResponse{
		Variables:          config.GetNotificationTemplateVariableCatalog(),
		ImageNameVariables: config.ImageNameTemplateVariables,
		ImageNameDefaults:  config.DefaultImageNameTemplates(),
	}

	httpx.SendResponse(w, ld, response)
}

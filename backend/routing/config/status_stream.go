package routes_config

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"net/http"
)

// StreamAppConfigStatus godoc
// @Summary      Stream Config Status
// @Description  Server-Sent Events stream of the app status. Sends the status on connect, then again whenever it changes, like each step while aura starts.
// @Tags         Config
// @Produce      text/event-stream
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Router       /api/config/stream [get]
func StreamAppConfigStatus(w http.ResponseWriter, r *http.Request) {
	streamConfigStatus(w, r, currentConfigStatus)
}

// StreamSetupConfigStatus godoc
// @Summary      Stream Config Status (Setup Mode)
// @Description  Server-Sent Events stream of the app status while aura starts or is in setup mode. Without the setup code, only the app status is sent, not the config.
// @Tags         Config
// @Produce      text/event-stream
// @Router       /api/config/stream [get]
func StreamSetupConfigStatus(w http.ResponseWriter, r *http.Request) {
	streamConfigStatus(w, r, setupConfigStatus)
}

func streamConfigStatus(w http.ResponseWriter, r *http.Request, status func(ctx context.Context) AppConfigStatus) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Stream Config Status", logging.LevelTrace)
	ctx = logging.WithCurrentAction(ctx, logAction)

	stream, ok := httpx.StartSSE(w)
	if !ok {
		return
	}

	changes := config.SubscribeStatus()
	defer config.UnsubscribeStatus(changes)

	send := func() { stream.Send(configStatusResponse{Status: status(ctx)}) }
	send()
	httpx.StreamEvents(ctx, stream, changes, func(struct{}) { send() })
}

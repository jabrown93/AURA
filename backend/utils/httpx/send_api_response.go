package httpx

import (
	"aura/logging"
	"encoding/json"
	"net/http"
)

type JSONResponse struct {
	Status string                `json:"status" example:"success"`
	Data   any                   `json:"data"`
	Error  *logging.LogErrorInfo `json:"error"`
}

type UnauthorizedResponse struct {
	Message string `json:"message" example:"Invalid or expired token"`
}

func SendResponse(w http.ResponseWriter, log *logging.LogData, data any) {
	sendResponse(w, log, data, 0)
}

func SendResponseWithStatus(w http.ResponseWriter, log *logging.LogData, data any, status int) {
	sendResponse(w, log, data, status)
}

func sendResponse(w http.ResponseWriter, log *logging.LogData, data any, status int) {
	var response JSONResponse
	// Always check for error actions
	for _, action := range log.Actions {
		action.Complete()
		if action.Status == logging.StatusError && action.Error != nil {
			response.Error = action.Error
			if log.Status == "" {
				log.Status = logging.StatusError
			}
			break
		}
	}

	if log.Status == "" {
		log.Status = logging.StatusSuccess
	}
	response.Status = log.Status
	response.Data = data

	if status == 0 {
		if log.Status == logging.StatusError {
			status = http.StatusInternalServerError
		} else {
			status = http.StatusOK
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

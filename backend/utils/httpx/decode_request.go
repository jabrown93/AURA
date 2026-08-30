package httpx

import (
	"aura/logging"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

func DecodeRequestBodyToJSON(ctx context.Context, r io.ReadCloser, v any, structName string) logging.LogErrorInfo {
	body, err := io.ReadAll(r)
	if err != nil {
		_, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Decoding request body into `%s` struct", structName), logging.LevelTrace)
		defer logAction.Complete()
		logAction.SetError("Failed to read request body", fmt.Sprintf("Ensure that the request body can be read for %s", structName),
			map[string]any{
				"error": err.Error(),
			})
		return *logAction.Error
	}
	defer r.Close()

	decoder := json.NewDecoder(bytes.NewReader(body))
	err = decoder.Decode(v)
	if err != nil {
		_, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Decoding request body into `%s` struct", structName), logging.LevelTrace)
		defer logAction.Complete()

		// This map is both logged and returned to the caller, and the body can carry
		// credentials (login POSTs, full config POSTs), so only its size is safe to report.
		errorDetails := map[string]any{
			"request_body_len": len(body),
			"error":            err.Error(),
			"error_type":       fmt.Sprintf("%T", err),
		}

		// Enhanced error reporting for JSON errors
		switch e := err.(type) {
		case *json.UnmarshalTypeError:
			errorDetails["field"] = e.Field
			errorDetails["value"] = e.Value
			errorDetails["type"] = e.Type.String()
			logAction.SetError(
				fmt.Sprintf("Type error for field '%s' in struct '%s'", e.Field, structName),
				fmt.Sprintf("Check the type of field '%s' (expected %s, got %s)", e.Field, e.Type.String(), e.Value),
				errorDetails,
			)
		case *json.SyntaxError:
			errorDetails["offset"] = e.Offset
			logAction.SetError(
				"JSON syntax error",
				fmt.Sprintf("Syntax error at offset %d in struct '%s'", e.Offset, structName),
				errorDetails,
			)
		default:
			logAction.SetError(
				"Failed to decode the JSON request body",
				fmt.Sprintf("Ensure that the JSON is correct for %s", structName),
				errorDetails,
			)
		}
		return *logAction.Error
	}
	return logging.LogErrorInfo{}
}

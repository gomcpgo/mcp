package server

import (
	"encoding/json"
	"log"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// transportProgressReporter is the server-side handler.ProgressReporter
// implementation. It forwards Report calls as notifications/progress on the
// active transport, carrying the progressToken that came in on the request.
type transportProgressReporter struct {
	sendNotification func(method string, params interface{}) error
	token            interface{}
}

func (r *transportProgressReporter) Report(progress float64, total *float64, message string) error {
	return r.sendNotification(protocol.NotificationProgress, protocol.ProgressParams{
		ProgressToken: r.token,
		Progress:      progress,
		Total:         total,
		Message:       message,
	})
}

// extractProgressToken returns the `_meta.progressToken` value from a
// request's params, or nil if absent. It tolerates malformed `_meta` silently
// so a bad metadata block never fails an otherwise-valid request.
func extractProgressToken(params json.RawMessage) interface{} {
	if len(params) == 0 {
		return nil
	}
	var envelope struct {
		Meta json.RawMessage `json:"_meta"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil || len(envelope.Meta) == 0 {
		return nil
	}
	var meta struct {
		ProgressToken interface{} `json:"progressToken"`
	}
	if err := json.Unmarshal(envelope.Meta, &meta); err != nil {
		log.Printf("malformed _meta on request; ignoring for progress: %v", err)
		return nil
	}
	return meta.ProgressToken
}

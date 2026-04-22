package protocol

// MCP logging levels per spec, listed low→high severity. Lower severity levels
// are chattier; the server emits a notifications/message only when the
// message's level is at or above the configured threshold.
const (
	LogLevelDebug     = "debug"
	LogLevelInfo      = "info"
	LogLevelNotice    = "notice"
	LogLevelWarning   = "warning"
	LogLevelError     = "error"
	LogLevelCritical  = "critical"
	LogLevelAlert     = "alert"
	LogLevelEmergency = "emergency"
)

// logLevelRank maps the spec level strings to an ordering where higher means
// more severe. Unknown strings collapse to -1 which is below every real level
// so a misconfigured threshold blocks emission rather than spamming.
var logLevelRank = map[string]int{
	LogLevelDebug:     0,
	LogLevelInfo:      1,
	LogLevelNotice:    2,
	LogLevelWarning:   3,
	LogLevelError:     4,
	LogLevelCritical:  5,
	LogLevelAlert:     6,
	LogLevelEmergency: 7,
}

// LogLevelRank returns the severity rank of level, or -1 if level is not one
// of the spec-defined strings.
func LogLevelRank(level string) int {
	if r, ok := logLevelRank[level]; ok {
		return r
	}
	return -1
}

// SetLevelParams is the payload of a logging/setLevel request.
type SetLevelParams struct {
	Level string `json:"level"`
}

// LogMessageParams is the payload of a notifications/message notification.
// Data is any JSON-serializable value per the spec.
type LogMessageParams struct {
	Level  string      `json:"level"`
	Logger string      `json:"logger,omitempty"`
	Data   interface{} `json:"data"`
}

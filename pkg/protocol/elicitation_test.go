package protocol

import (
	"encoding/json"
	"testing"
)

func TestElicitationMethodConstant(t *testing.T) {
	if MethodElicitationCreate != "elicitation/create" {
		t.Errorf("MethodElicitationCreate = %q, want %q", MethodElicitationCreate, "elicitation/create")
	}
}

func TestElicitationActionConstants(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{ElicitationActionAccept, "accept"},
		{ElicitationActionDecline, "decline"},
		{ElicitationActionCancel, "cancel"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestElicitationRequestParamsMarshal(t *testing.T) {
	params := ElicitationRequestParams{
		Message:         "Please confirm",
		RequestedSchema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`),
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if parsed["message"] != "Please confirm" {
		t.Errorf("message = %v, want 'Please confirm'", parsed["message"])
	}
	schema, ok := parsed["requestedSchema"].(map[string]interface{})
	if !ok {
		t.Fatalf("requestedSchema missing or wrong type: %T", parsed["requestedSchema"])
	}
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want 'object'", schema["type"])
	}
}

func TestElicitationResultAcceptMarshal(t *testing.T) {
	result := ElicitationResult{
		Action:  ElicitationActionAccept,
		Content: map[string]interface{}{"name": "Alice", "age": 30},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if parsed["action"] != "accept" {
		t.Errorf("action = %v, want accept", parsed["action"])
	}
	content, ok := parsed["content"].(map[string]interface{})
	if !ok {
		t.Fatalf("content missing or wrong type")
	}
	if content["name"] != "Alice" {
		t.Errorf("content.name = %v, want Alice", content["name"])
	}
}

func TestElicitationResultDeclineOmitsContent(t *testing.T) {
	result := ElicitationResult{Action: ElicitationActionDecline}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if parsed["action"] != "decline" {
		t.Errorf("action = %v, want decline", parsed["action"])
	}
	if _, present := parsed["content"]; present {
		t.Error("content should be omitted when action != accept")
	}
}

func TestClientCapabilitiesElicitationMarshal(t *testing.T) {
	caps := ClientCapabilities{
		Elicitation: &ElicitationClientCapabilities{},
	}

	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	elicit, ok := parsed["elicitation"].(map[string]interface{})
	if !ok {
		t.Fatalf("elicitation missing or wrong type: %T", parsed["elicitation"])
	}
	if len(elicit) != 0 {
		t.Errorf("elicitation capability should marshal as empty object, got %v", elicit)
	}
}

func TestClientCapabilitiesElicitationOmittedWhenNil(t *testing.T) {
	caps := ClientCapabilities{}

	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, present := parsed["elicitation"]; present {
		t.Error("elicitation should be omitted when nil")
	}
}

func TestElicitationRequestParamsUnmarshal(t *testing.T) {
	input := `{
		"message": "Please fill in",
		"requestedSchema": {"type":"object","properties":{"email":{"type":"string","format":"email"}}}
	}`

	var params ElicitationRequestParams
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if params.Message != "Please fill in" {
		t.Errorf("Message = %q, want 'Please fill in'", params.Message)
	}
	if len(params.RequestedSchema) == 0 {
		t.Error("RequestedSchema should be populated")
	}
}

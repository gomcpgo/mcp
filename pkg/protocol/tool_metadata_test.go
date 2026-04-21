package protocol

import (
	"encoding/json"
	"testing"
)

// TestToolMarshalling_OmitsEmptyOptionalFields asserts that a Tool with only
// the required fields set does not emit any of the new optional keys.
// Backward-compat: older consumers must not see new fields unless the server
// explicitly populates them.
func TestToolMarshalling_OmitsEmptyOptionalFields(t *testing.T) {
	tool := Tool{
		Name:        "basic",
		Description: "no extras",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"title", "annotations", "outputSchema", "icons", "_meta"} {
		if _, present := parsed[key]; present {
			t.Errorf("unexpected key %q present when value is zero", key)
		}
	}
}

// TestToolMarshalling_IncludesAnnotationsWhenSet verifies that explicit hint
// values (including false) round-trip via the *bool fields.
func TestToolMarshalling_IncludesAnnotationsWhenSet(t *testing.T) {
	readOnly := true
	destructive := false
	tool := Tool{
		Name:        "annotated",
		Title:       "Annotated Tool",
		Description: "has hints",
		InputSchema: json.RawMessage(`{}`),
		Annotations: &ToolAnnotations{
			Title:           "Ignored Alias",
			ReadOnlyHint:    &readOnly,
			DestructiveHint: &destructive,
		},
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["title"] != "Annotated Tool" {
		t.Errorf("title = %v, want %q", parsed["title"], "Annotated Tool")
	}

	ann, ok := parsed["annotations"].(map[string]interface{})
	if !ok {
		t.Fatalf("annotations missing or wrong type: %v", parsed["annotations"])
	}
	if ann["readOnlyHint"] != true {
		t.Errorf("readOnlyHint = %v, want true", ann["readOnlyHint"])
	}
	if ann["destructiveHint"] != false {
		t.Errorf("destructiveHint = %v, want false", ann["destructiveHint"])
	}
	// Unset hints must NOT appear so servers can distinguish "no hint" from "false".
	if _, present := ann["idempotentHint"]; present {
		t.Error("idempotentHint should be omitted when unset")
	}
	if _, present := ann["openWorldHint"]; present {
		t.Error("openWorldHint should be omitted when unset")
	}
}

// TestToolMarshalling_IncludesIconsWhenSet verifies icons round-trip.
func TestToolMarshalling_IncludesIconsWhenSet(t *testing.T) {
	tool := Tool{
		Name:        "iconed",
		Description: "has icons",
		InputSchema: json.RawMessage(`{}`),
		Icons: []Icon{
			{Src: "data:image/png;base64,AAA", MimeType: "image/png", Sizes: "48x48"},
			{Src: "https://example.com/icon.svg", MimeType: "image/svg+xml"},
		},
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	icons, ok := parsed["icons"].([]interface{})
	if !ok {
		t.Fatalf("icons missing or wrong type: %v", parsed["icons"])
	}
	if len(icons) != 2 {
		t.Fatalf("len(icons) = %d, want 2", len(icons))
	}
	first := icons[0].(map[string]interface{})
	if first["src"] != "data:image/png;base64,AAA" {
		t.Errorf("icons[0].src = %v", first["src"])
	}
	if first["sizes"] != "48x48" {
		t.Errorf("icons[0].sizes = %v, want 48x48", first["sizes"])
	}
	second := icons[1].(map[string]interface{})
	if _, hasSizes := second["sizes"]; hasSizes {
		t.Error("icons[1].sizes should be omitted when empty")
	}
}

// TestToolAnnotations_NilHintsDistinctFromFalse confirms that an explicit
// false does not round-trip as "unset" and vice versa.
func TestToolAnnotations_NilHintsDistinctFromFalse(t *testing.T) {
	falseVal := false
	trueVal := true

	withFalse := &ToolAnnotations{ReadOnlyHint: &falseVal}
	withTrue := &ToolAnnotations{ReadOnlyHint: &trueVal}
	withNil := &ToolAnnotations{}

	falseBytes, _ := json.Marshal(withFalse)
	trueBytes, _ := json.Marshal(withTrue)
	nilBytes, _ := json.Marshal(withNil)

	if string(falseBytes) == string(nilBytes) {
		t.Error("explicit false must serialise differently from unset")
	}
	if string(trueBytes) == string(nilBytes) {
		t.Error("explicit true must serialise differently from unset")
	}

	// Round-trip unset → decoded Annotations should have nil pointer.
	var decoded ToolAnnotations
	if err := json.Unmarshal(nilBytes, &decoded); err != nil {
		t.Fatalf("unmarshal nil: %v", err)
	}
	if decoded.ReadOnlyHint != nil {
		t.Errorf("decoded ReadOnlyHint = %v, want nil", *decoded.ReadOnlyHint)
	}

	// Round-trip explicit false → decoded should be non-nil and false.
	var decodedFalse ToolAnnotations
	if err := json.Unmarshal(falseBytes, &decodedFalse); err != nil {
		t.Fatalf("unmarshal false: %v", err)
	}
	if decodedFalse.ReadOnlyHint == nil {
		t.Fatal("explicit false round-trip produced nil pointer")
	}
	if *decodedFalse.ReadOnlyHint != false {
		t.Errorf("round-tripped value = %v, want false", *decodedFalse.ReadOnlyHint)
	}
}

// TestCallToolResponse_MarshalStructuredContent covers both presence and
// absence of the new structuredContent field.
func TestCallToolResponse_MarshalStructuredContent(t *testing.T) {
	withStructured := CallToolResponse{
		Content: []ToolContent{{Type: "text", Text: "hi"}},
		StructuredContent: map[string]interface{}{
			"temperature": 72,
			"humidity":    0.43,
		},
	}
	without := CallToolResponse{
		Content: []ToolContent{{Type: "text", Text: "hi"}},
	}

	withData, err := json.Marshal(withStructured)
	if err != nil {
		t.Fatalf("marshal with: %v", err)
	}
	withoutData, err := json.Marshal(without)
	if err != nil {
		t.Fatalf("marshal without: %v", err)
	}

	var parsedWith map[string]interface{}
	if err := json.Unmarshal(withData, &parsedWith); err != nil {
		t.Fatalf("unmarshal with: %v", err)
	}
	sc, ok := parsedWith["structuredContent"].(map[string]interface{})
	if !ok {
		t.Fatalf("structuredContent missing or wrong type: %v", parsedWith["structuredContent"])
	}
	// JSON numbers decode as float64.
	if sc["temperature"].(float64) != 72 {
		t.Errorf("structuredContent.temperature = %v, want 72", sc["temperature"])
	}

	var parsedWithout map[string]interface{}
	if err := json.Unmarshal(withoutData, &parsedWithout); err != nil {
		t.Fatalf("unmarshal without: %v", err)
	}
	if _, present := parsedWithout["structuredContent"]; present {
		t.Error("structuredContent should be omitted when nil")
	}
}

package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/r3labs/sse/v2"
	"github.com/rs/zerolog"
)

// mockSSE is a simple mock for sse.Server
type mockSSE struct {
	lastPublishedEvent *sse.Event
	lastPublishedTopic string
}

// Publish implements the SSEPublisher interface for mockSSE
func (m *mockSSE) Publish(topic string, event *sse.Event) {
	m.lastPublishedTopic = topic
	m.lastPublishedEvent = event
}

func TestNewSSEWriter(t *testing.T) {
	var mockSrv SSEPublisher = &mockSSE{} // Use the interface type
	writer := NewSSEWriter(mockSrv)

	if writer.SSE != mockSrv { // This comparison should now be valid
		t.Errorf("Expected SSE server to be set")
	}
	if writer.TimeFormat != defaultTimeFormat {
		t.Errorf("Expected default TimeFormat, got %s", writer.TimeFormat)
	}
	if len(writer.PartsOrder) != len(defaultPartsOrder()) {
		t.Errorf("Expected default PartsOrder")
	}
}

func TestNewSSEWriter_WithOptions(t *testing.T) {
	mockSrv := &mockSSE{}
	customTimeFormat := "2006-01-02"
	customPartsOrder := []string{zerolog.LevelFieldName}

	writer := NewSSEWriter(mockSrv, func(w *SSEWriter) {
		w.TimeFormat = customTimeFormat
		w.PartsOrder = customPartsOrder
	})

	if writer.TimeFormat != customTimeFormat {
		t.Errorf("Expected custom TimeFormat, got %s", writer.TimeFormat)
	}
	if len(writer.PartsOrder) != 1 || writer.PartsOrder[0] != zerolog.LevelFieldName {
		t.Errorf("Expected custom PartsOrder")
	}
}

func TestLogMessage_Bytes(t *testing.T) {
	lm := LogMessage{Time: "12:00", Level: "INF", Message: "hello"}
	data, err := lm.Bytes()
	if err != nil {
		t.Fatalf("Bytes() failed: %v", err)
	}

	var decoded LogMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal bytes: %v", err)
	}

	if decoded.Time != lm.Time || decoded.Level != lm.Level || decoded.Message != lm.Message {
		t.Errorf("Decoded message mismatch. Got %+v, want %+v", decoded, lm)
	}
}

func TestSSEWriter_Write_NilSSE(t *testing.T) {
	writer := SSEWriter{SSE: nil}
	n, err := writer.Write([]byte(`{"level":"info","message":"test"}`))
	if err != nil {
		t.Errorf("Write() with nil SSE should not error, got %v", err)
	}
	if n != 0 {
		t.Errorf("Write() with nil SSE should return 0 bytes written, got %d", n)
	}
}

func TestSSEWriter_Write_InvalidJSON(t *testing.T) {
	mockSrv := &mockSSE{}
	writer := NewSSEWriter(mockSrv)
	_, err := writer.Write([]byte(`invalid json`))
	if err == nil {
		t.Error("Write() with invalid JSON should error")
	}
}

func TestSSEWriter_Write_Successful(t *testing.T) {
	mockSrv := &mockSSE{}
	writer := NewSSEWriter(mockSrv)

	logTime := time.Now()
	logEvent := map[string]interface{}{
		zerolog.TimestampFieldName: logTime.Format(zerolog.TimeFieldFormat),
		zerolog.LevelFieldName:     zerolog.LevelInfoValue,
		zerolog.MessageFieldName:   "test message",
		"custom_field":             "custom_value",
		zerolog.CallerFieldName:    "main.go:123",
	}
	jsonData, _ := json.Marshal(logEvent)

	n, err := writer.Write(jsonData)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}
	if n != len(jsonData) {
		t.Errorf("Write() returned %d bytes, want %d", n, len(jsonData))
	}

	if mockSrv.lastPublishedTopic != "logs" {
		t.Errorf("Expected topic 'logs', got '%s'", mockSrv.lastPublishedTopic)
	}
	if mockSrv.lastPublishedEvent == nil {
		t.Fatal("Expected event to be published")
	}

	var publishedMsg LogMessage
	if err := json.Unmarshal(mockSrv.lastPublishedEvent.Data, &publishedMsg); err != nil {
		t.Fatalf("Failed to unmarshal published data: %v", err)
	}

	if publishedMsg.Level != "INF" {
		t.Errorf("Expected published level 'INF', got '%s'", publishedMsg.Level)
	}
	if !strings.Contains(publishedMsg.Message, "test message") {
		t.Errorf("Published message does not contain original message. Got: %s", publishedMsg.Message)
	}
	if !strings.Contains(publishedMsg.Message, "custom_field=custom_value") {
		t.Errorf("Published message does not contain custom field. Got: %s", publishedMsg.Message)
	}
	if !strings.Contains(publishedMsg.Message, "main.go:123 >") {
		t.Errorf("Published message does not contain caller. Got: %s", publishedMsg.Message)
	}
	// Time formatting is complex, check if it's present
	if publishedMsg.Time == "" {
		t.Error("Published message time is empty")
	}
}

func TestNeedsQuote(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"simple", false},
		{"with space", true},
		{"with\"quote", true},
		{"with\\escape", true},
		{"with\x1fcontrol", true},
	}
	for _, tt := range tests {
		if got := needsQuote(tt.input); got != tt.want {
			t.Errorf("needsQuote(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDefaultFormatters(t *testing.T) {
	// Test defaultFormatTimestamp
	tsFormatter := defaultFormatTimestamp(time.RFC3339)
	now := time.Now()
	formattedTime := tsFormatter(now.Format(zerolog.TimeFieldFormat))
	parsedTime, _ := time.Parse(time.RFC3339, formattedTime)
	if !parsedTime.Local().Equal(now.Local().Truncate(time.Second)) { // Truncate for comparison
		t.Errorf("defaultFormatTimestamp: expected %v, got %v", now.Local().Format(time.RFC3339), formattedTime)
	}
	formattedNumTime := tsFormatter(json.Number(fmt.Sprintf("%d", now.Unix())))
	parsedNumTime, _ := time.Parse(time.RFC3339, formattedNumTime)
	if !parsedNumTime.Local().Equal(now.Local().Truncate(time.Second)) {
		t.Errorf("defaultFormatTimestamp with number: expected %v, got %v", now.Local().Format(time.RFC3339), formattedNumTime)
	}


	// Test defaultFormatLevel
	levelFormatter := defaultFormatLevel()
	if levelFormatter(zerolog.LevelInfoValue) != "INF" {
		t.Errorf("defaultFormatLevel: expected INF, got %s", levelFormatter(zerolog.LevelInfoValue))
	}
	if levelFormatter("unknown") != "unknown" { // As per current logic, it returns the input if not a known level string
		t.Errorf("defaultFormatLevel for unknown: expected unknown, got %s", levelFormatter("unknown"))
	}
	if levelFormatter(nil) != "???" {
		t.Errorf("defaultFormatLevel for nil: expected ???, got %s", levelFormatter(nil))
	}


	// Test defaultFormatCaller
	callerFormatter := defaultFormatCaller()
	// This is hard to test precisely without mocking os.Getwd and filepath.Rel
	// We'll check for basic formatting.
	if !strings.HasSuffix(callerFormatter("path/to/file.go:123"), "path/to/file.go:123 >") {
		t.Errorf("defaultFormatCaller: unexpected format %s", callerFormatter("path/to/file.go:123"))
	}

	// Test defaultFormatMessage
	if defaultFormatMessage("hello") != "hello" {
		t.Errorf("defaultFormatMessage: expected hello, got %s", defaultFormatMessage("hello"))
	}
	if defaultFormatMessage(nil) != "" {
		t.Errorf("defaultFormatMessage for nil: expected empty string, got %s", defaultFormatMessage(nil))
	}

	// Test defaultFormatFieldName
	fieldNameFormatter := defaultFormatFieldName()
	if fieldNameFormatter("field") != "field=" {
		t.Errorf("defaultFormatFieldName: expected field=, got %s", fieldNameFormatter("field"))
	}

	// Test defaultFormatFieldValue
	if defaultFormatFieldValue("value") != "value" {
		t.Errorf("defaultFormatFieldValue: expected value, got %s", defaultFormatFieldValue("value"))
	}
	
	// Test defaultFormatErrFieldName
	errFieldNameFormatter := defaultFormatErrFieldName()
	if errFieldNameFormatter("error") != "error=" {
		t.Errorf("defaultFormatErrFieldName: expected error=, got %s", errFieldNameFormatter("error"))
	}

	// Test defaultFormatErrFieldValue
	errFieldValueFormatter := defaultFormatErrFieldValue()
	if errFieldValueFormatter("err_value") != "err_value=" { // Note: current implementation adds "="
		t.Errorf("defaultFormatErrFieldValue: expected err_value=, got %s", errFieldValueFormatter("err_value"))
	}
}

func TestWritePart(t *testing.T) {
	mockSrv := &mockSSE{}
	writer := NewSSEWriter(mockSrv)
	buf := new(bytes.Buffer)
	evt := map[string]interface{}{
		zerolog.TimestampFieldName: time.Now().Format(zerolog.TimeFieldFormat),
		zerolog.LevelFieldName:     zerolog.LevelDebugValue,
		zerolog.MessageFieldName:   "debug message",
		zerolog.CallerFieldName:    "test.go:42",
		"custom":                   "val",
	}

	writer.writePart(buf, evt, zerolog.LevelFieldName)
	if !strings.Contains(buf.String(), "DBG") {
		t.Errorf("writePart for level did not write DBG. Got: %s", buf.String())
	}
	buf.Reset()

	writer.writePart(buf, evt, zerolog.MessageFieldName)
	if !strings.Contains(buf.String(), "debug message") {
		t.Errorf("writePart for message did not write 'debug message'. Got: %s", buf.String())
	}
	buf.Reset()

	writer.writePart(buf, evt, zerolog.CallerFieldName)
	if !strings.Contains(buf.String(), "test.go:42 >") {
		t.Errorf("writePart for caller did not write 'test.go:42 >'. Got: %s", buf.String())
	}
	buf.Reset()
	
	// Test with a part not in default formatters (should use defaultFormatFieldValue)
	writer.writePart(buf, evt, "custom")
	if !strings.Contains(buf.String(), "val") {
		t.Errorf("writePart for custom field did not write 'val'. Got: %s", buf.String())
	}
}

func TestWriteFields(t *testing.T) {
	mockSrv := &mockSSE{}
	writer := NewSSEWriter(mockSrv)
	buf := new(bytes.Buffer)
	evt := map[string]interface{}{
		zerolog.TimestampFieldName: time.Now().Format(zerolog.TimeFieldFormat),
		zerolog.LevelFieldName:     zerolog.LevelWarnValue,
		zerolog.MessageFieldName:   "warning message",
		"field1":                   "value1",
		"another_field":            123,
		zerolog.ErrorFieldName:     "some error",
	}
	
	writer.writeFields(buf, evt)
	output := buf.String()

	// Check if error field is present and formatted
	// The error value "some error" contains a space, so it should be quoted.
	// defaultFormatErrFieldValue adds a trailing "="
	expectedErrorField := fmt.Sprintf("%s=%q=", zerolog.ErrorFieldName, "some error") // "error=\"some error\"="
	if !strings.Contains(output, expectedErrorField) {
		t.Errorf("writeFields did not correctly format error field. Expected to contain '%s'. Got: %s", expectedErrorField, output)
	}
	// Check if other fields are present and formatted
	if !strings.Contains(output, "field1=value1") {
		t.Errorf("writeFields did not correctly format field1. Got: %s", output)
	}
	if !strings.Contains(output, "another_field=123") {
		t.Errorf("writeFields did not correctly format another_field. Got: %s", output)
	}
	// Check that standard zerolog fields (level, timestamp, message) are NOT in this output
	if strings.Contains(output, zerolog.LevelWarnValue) {
		t.Errorf("writeFields incorrectly included level field. Got: %s", output)
	}
}
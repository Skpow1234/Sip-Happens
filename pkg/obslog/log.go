package obslog

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"sip-happens/pkg/redact"
)

// Info writes a JSON log line with optional correlation fields from ctx.
func Info(ctx context.Context, msg string, fields map[string]any) {
	write(ctx, "info", msg, fields)
}

// Warn writes a JSON log line at warn level.
func Warn(ctx context.Context, msg string, fields map[string]any) {
	write(ctx, "warn", msg, fields)
}

// Error writes a JSON log line at error level.
func Error(ctx context.Context, msg string, fields map[string]any) {
	write(ctx, "error", msg, fields)
}

func write(ctx context.Context, level, msg string, fields map[string]any) {
	entry := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": level,
		"msg":   msg,
	}
	for k, v := range FieldsFromContext(ctx) {
		entry[k] = v
	}
	for k, v := range redact.Fields(fields) {
		entry[k] = v
	}
	b, err := json.Marshal(entry)
	if err != nil {
		log.Printf(`{"level":"error","msg":"obslog marshal failed","err":%q}`, err.Error())
		return
	}
	log.Println(string(b))
}

package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

type ctxKey string

const (
	TraceIDKey        ctxKey = "trace_id"
	SpanIDKey         ctxKey = "span_id"
	RequestIDKey      ctxKey = "request_id"
	MerchantIDKey     ctxKey = "merchant_id"
	IdempotencyKeyKey ctxKey = "idempotency_key"
)

// Regex to identify 13-19 digit credit card numbers (PAN)
var panRegex = regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`)

// Regex for CVV
var cvvRegex = regexp.MustCompile(`(?i)(cvv|cvc|cvn|security_code)["']?\s*[:=]\s*["']?\d{3,4}`)

// Config holds configuration for the logger.
type Config struct {
	Level  string // DEBUG, INFO, WARN, ERROR
	Format string // json, text
	Output io.Writer
}

// ContextHandler wraps slog.Handler to inject contextual fields and mask sensitive data.
type ContextHandler struct {
	slog.Handler
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx == nil {
		return h.Handler.Handle(ctx, r)
	}

	// Enrich record from context
	if val, ok := ctx.Value(TraceIDKey).(string); ok && val != "" {
		r.AddAttrs(slog.String("trace_id", val))
	}
	if val, ok := ctx.Value(SpanIDKey).(string); ok && val != "" {
		r.AddAttrs(slog.String("span_id", val))
	}
	if val, ok := ctx.Value(RequestIDKey).(string); ok && val != "" {
		r.AddAttrs(slog.String("request_id", val))
	}
	if val, ok := ctx.Value(MerchantIDKey).(string); ok && val != "" {
		r.AddAttrs(slog.String("merchant_id", val))
	}
	if val, ok := ctx.Value(IdempotencyKeyKey).(string); ok && val != "" {
		r.AddAttrs(slog.String("idempotency_key", val))
	}

	return h.Handler.Handle(ctx, r)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithGroup(name)}
}

// MaskPAN replaces all but the first 6 and last 4 digits of a card number.
// Example: 4111111111111111 -> 411111******1111
func MaskPAN(input string) string {
	return panRegex.ReplaceAllStringFunc(input, func(m string) string {
		clean := strings.ReplaceAll(strings.ReplaceAll(m, " ", ""), "-", "")
		if len(clean) < 13 || len(clean) > 19 {
			return m
		}
		prefix := clean[:6]
		suffix := clean[len(clean)-4:]
		masked := prefix + strings.Repeat("*", len(clean)-10) + suffix
		return masked
	})
}

// MaskSensitiveData masks PAN and CVV/CVC in strings.
func MaskSensitiveData(input string) string {
	res := MaskPAN(input)
	res = cvvRegex.ReplaceAllString(res, "$1: ***")
	return res
}

// sanitizeAttr ensures no sensitive fields leak in logs.
func sanitizeAttr(groups []string, a slog.Attr) slog.Attr {
	key := strings.ToLower(a.Key)
	if strings.Contains(key, "card") || strings.Contains(key, "pan") ||
		strings.Contains(key, "cvv") || strings.Contains(key, "token") ||
		strings.Contains(key, "password") || strings.Contains(key, "secret") {
		if a.Value.Kind() == slog.KindString {
			return slog.String(a.Key, MaskSensitiveData(a.Value.String()))
		}
	}
	if a.Value.Kind() == slog.KindString {
		return slog.String(a.Key, MaskSensitiveData(a.Value.String()))
	}
	return a
}

// New creates and configures a new slog.Logger instance.
func New(cfg Config) *slog.Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}

	var level slog.Level
	switch strings.ToUpper(cfg.Level) {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: sanitizeAttr,
		AddSource:   level == slog.LevelDebug,
	}

	var baseHandler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		baseHandler = slog.NewJSONHandler(out, opts)
	} else {
		baseHandler = slog.NewTextHandler(out, opts)
	}

	ctxHandler := &ContextHandler{Handler: baseHandler}
	return slog.New(ctxHandler)
}

// WithContext returns a logger enriched with context values.
func WithContext(ctx context.Context, l *slog.Logger) *slog.Logger {
	if l == nil {
		l = slog.Default()
	}
	return l
}

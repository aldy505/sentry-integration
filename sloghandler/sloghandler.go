package sloghandler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
)

type SentrySlogHandler struct {
	attributes []slog.Attr
	group      string
	level      slog.Level
}

func slogAttrToSentryAttr(a slog.Attr) []attribute.Builder {
	switch a.Value.Kind() {
	case slog.KindAny:
		// Does it implements Stringer?
		if s, ok := a.Value.Any().(fmt.Stringer); ok {
			return []attribute.Builder{attribute.String(a.Key, s.String())}
		}

		// Does is implements Error?
		if e, ok := a.Value.Any().(error); ok {
			return []attribute.Builder{attribute.String(a.Key, e.Error())}
		}

		// Does it implement json.Marshaler?
		if m, ok := a.Value.Any().(json.Marshaler); ok {
			out, err := m.MarshalJSON()
			if err == nil {
				return []attribute.Builder{attribute.String(a.Key, string(out))}
			}
		}

		// Does it implement log.Valuer?
		if v, ok := a.Value.Any().(slog.LogValuer); ok {
			return []attribute.Builder{attribute.String(a.Key, v.LogValue().String())}
		}

		// If none of the above, we can't process it.
		return []attribute.Builder{}
	case slog.KindBool:
		return []attribute.Builder{attribute.Bool(a.Key, a.Value.Bool())}
	case slog.KindDuration:
		return []attribute.Builder{attribute.String(a.Key, a.Value.Duration().String())}
	case slog.KindFloat64:
		return []attribute.Builder{attribute.Float64(a.Key, a.Value.Float64())}
	case slog.KindInt64:
		return []attribute.Builder{attribute.Int64(a.Key, a.Value.Int64())}
	case slog.KindString:
		return []attribute.Builder{attribute.String(a.Key, a.Value.String())}
	case slog.KindTime:
		return []attribute.Builder{attribute.String(a.Key, a.Value.Time().Format(time.RFC3339))}
	case slog.KindUint64:
		return []attribute.Builder{attribute.Int64(a.Key, int64(a.Value.Uint64()))}
	case slog.KindGroup:
		attr := a.Value.Group()
		attrs := make([]attribute.Builder, 0)
		for _, attr := range attr {
			attrs = append(attrs, slogAttrToSentryAttr(attr)...)
		}
		return attrs
	case slog.KindLogValuer:
		return []attribute.Builder{attribute.String(a.Key, a.Value.LogValuer().LogValue().String())}
	}

	return []attribute.Builder{}
}

// Enabled implements slog.Handler.
func (s *SentrySlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= s.level
}

// Handle implements slog.Handler.
func (s *SentrySlogHandler) Handle(ctx context.Context, record slog.Record) error {
	logger := sentry.NewLogger(ctx)

	sentryAttributes := make([]attribute.Builder, 0)
	for _, attr := range s.attributes {
		sentryAttributes = append(sentryAttributes, slogAttrToSentryAttr(attr)...)
	}
	if s.group != "" {
		sentryAttributes = append(sentryAttributes, attribute.String("group", s.group))
	}
	record.Attrs(func(a slog.Attr) bool {
		sentryAttributes = append(sentryAttributes, slogAttrToSentryAttr(a)...)
		return true
	})

	logger.SetAttributes(sentryAttributes...)

	switch record.Level {
	case slog.LevelDebug:
		logger.Debug(ctx, record.Message)
	case slog.LevelInfo:
		logger.Info(ctx, record.Message)
	case slog.LevelWarn:
		logger.Warn(ctx, record.Message)
	case slog.LevelError:
		logger.Error(ctx, record.Message)
	}

	return nil
}

// WithAttrs implements slog.Handler.
func (s *SentrySlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	s.attributes = append(s.attributes, attrs...)
	return s
}

// WithGroup implements slog.Handler.
func (s *SentrySlogHandler) WithGroup(name string) slog.Handler {
	s.group = name
	return s
}

var _ slog.Handler = (*SentrySlogHandler)(nil)

func NewSentrySlogHandler(logLevel slog.Level) slog.Handler {
	return &SentrySlogHandler{
		attributes: make([]slog.Attr, 0),
		group:      "",
		level:      logLevel,
	}
}

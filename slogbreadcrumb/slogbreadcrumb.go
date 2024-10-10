// slogbreadcrumb sends everything that's emitted to slog as Sentry breadcrumb, rather than Sentry event (or error).
// Best used in conjunction with "github.com/samber/slog-multi" package.
//
// Example usage:
//
//	package main
//
//	import "log/slog"
//	import slogmulti "github.com/samber/slog-multi"
//	import slogbreadcrumb "github.com/aldy505/sentry-integration/slogbreadcrumb"
//
//	func main() {
//		slog.SetDefault(slog.New(slogmulti.Fanout(
//			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
//			&slogbreadcrumb.Handler{Enable: true, Level: slog.LevelDebug},
//		)))
//	}
package slogbreadcrumb

import (
	"context"
	"log/slog"

	"github.com/getsentry/sentry-go"
)

type Handler struct {
	Enable     bool
	Level      slog.Level
	attributes []slog.Attr
	group      string
}

func toSentryLevel(level slog.Level) sentry.Level {
	switch level {
	case slog.LevelDebug:
		return sentry.LevelDebug
	case slog.LevelInfo:
		return sentry.LevelInfo
	case slog.LevelWarn:
		return sentry.LevelWarning
	case slog.LevelError:
		return sentry.LevelError
	default:
		return sentry.LevelInfo
	}
}

func (s *Handler) Enabled(_ context.Context, level slog.Level) bool {
	if !s.Enable {
		return false
	}

	return level >= s.Level
}

func (s *Handler) Handle(ctx context.Context, record slog.Record) error {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		return nil
	}

	var data = make(map[string]any)
	for _, attr := range s.attributes {
		data[attr.Key] = attr.Value
	}
	if s.group != "" {
		data["group"] = s.group
	}

	hub.AddBreadcrumb(&sentry.Breadcrumb{
		Type:      "log",
		Category:  "log",
		Message:   record.Message,
		Data:      data,
		Level:     toSentryLevel(record.Level),
		Timestamp: record.Time,
	}, nil)

	return nil
}

func (s *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		Enable:     s.Enable,
		Level:      s.Level,
		attributes: append(s.attributes, attrs...),
		group:      s.group,
	}
}

func (s *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		Enable:     s.Enable,
		Level:      s.Level,
		attributes: s.attributes,
		group:      name,
	}
}

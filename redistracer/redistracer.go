// Package redistracer provides a tracer implementation for go-redis.
//
//	rdb := redis.NewClient(&redis.Options{
//		Addr: ":6379",
//	})
//	rdb.AddHook(redistracer.NewwSentryRedisTracer())
package redistracer

import (
	"context"
	"net"
	"strings"

	"github.com/getsentry/sentry-go"
	redis "github.com/redis/go-redis/v9"
)

type SentryRedisTracerOption func(*SentryRedisTracer)

func WithTags(tags map[string]string) SentryRedisTracerOption {
	return func(t *SentryRedisTracer) {
		for k, v := range tags {
			t.tags[k] = v
		}
	}
}

func WithTag(key, value string) SentryRedisTracerOption {
	return func(t *SentryRedisTracer) {
		t.tags[key] = value
	}
}

func NewSentryRedisTracer(opts ...SentryRedisTracerOption) redis.Hook {
	t := &SentryRedisTracer{}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

type SentryRedisTracer struct {
	network string
	addr    string

	tags map[string]string
}

// DialHook implements redis.Hook.
func (s *SentryRedisTracer) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		s.network = network
		s.addr = addr
		return next(ctx, network, addr)
	}
}

// ProcessHook implements redis.Hook.
func (s *SentryRedisTracer) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		span := sentry.StartSpan(ctx, "db.redis", sentry.WithTransactionName(strings.ToUpper(cmd.Name())), sentry.WithDescription(strings.ToUpper(cmd.Name())))
		if span == nil {
			return next(ctx, cmd)
		}
		span.SetData("db.system", "redis")
		span.SetData("db.operation", cmd.FullName())
		span.SetData("server.address", s.addr)

		for k, v := range s.tags {
			span.SetTag(k, v)
		}

		defer span.Finish()

		err := next(ctx, cmd)
		if err != nil {
			span.Status = sentry.SpanStatusInternalError
		}

		return err
	}
}

// ProcessPipelineHook implements redis.Hook.
func (s *SentryRedisTracer) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		span := sentry.StartSpan(ctx, "db.redis", sentry.WithTransactionName("PIPELINE"), sentry.WithDescription("PIPELINE"))
		if span == nil {
			return next(ctx, cmds)
		}
		span.SetData("db.system", "redis")
		span.SetData("db.operation", "PIPELINE")
		span.SetData("server.address", s.addr)
		defer span.Finish()

		err := next(ctx, cmds)
		if err != nil {
			span.Status = sentry.SpanStatusInternalError
		}

		return err
	}
}

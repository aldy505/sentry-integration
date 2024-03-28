// Package pgxtracer provides a tracer implementation for pgx.
//
//	databaseConfig, err := pgxpool.ParseConfig(fmt.Sprintf(
//		"user=%s password=%s host=%s port=%d dbname=%s sslmode=disable pool_max_conn_lifetime=15m pool_max_conn_idle_time=30m pool_health_check_period=1m",
//		configuration.Database.User,
//		configuration.Database.Password,
//		configuration.Database.Hostname,
//		configuration.Database.Port,
//		configuration.Database.Database))
//	if err != nil {
//		return fmt.Errorf("parsing database configuration: %w", err)
//	}
//
//	databaseConf := databaseConfig.Copy()
//
//	databaseConf.ConnConfig.Tracer = pgxtracer.NewSentryPgxTracer()
//
//	database, err := pgxpool.NewWithConfig(c.Context, databaseConf)
//	if err != nil {
//		return fmt.Errorf("connecting to database: %w", err)
//	}
//	defer database.Close()
package pgxtracer

import (
	"context"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
)

type SentryPgxTracerOption func(*Tracer)

func WithTags(tags map[string]string) SentryPgxTracerOption {
	return func(t *Tracer) {
		for k, v := range tags {
			t.tags[k] = v
		}
	}
}

func WithTag(key, value string) SentryPgxTracerOption {
	return func(t *Tracer) {
		t.tags[key] = value
	}
}

func NewSentryPgxTracer(opts ...SentryPgxTracerOption) pgx.QueryTracer {
	t := &Tracer{
		tags: make(map[string]string),
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

type Tracer struct {
	tags map[string]string
}

func (t Tracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	span := sentry.StartSpan(ctx, "db.sql.query", sentry.WithTransactionName(data.SQL), sentry.WithDescription(data.SQL))
	if span == nil {
		return ctx
	}
	span.SetData("db.system", "postgresql")

	return span.Context()
}

func (t Tracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	span := sentry.SpanFromContext(ctx)
	if span == nil {
		return
	}

	for k, v := range t.tags {
		span.SetTag(k, v)
	}

	if data.CommandTag.Insert() {
		span.SetData("db.operation", "INSERT")
	} else if data.CommandTag.Select() {
		span.SetData("db.operation", "SELECT")
	} else if data.CommandTag.Delete() {
		span.SetData("db.operation", "DELETE")
	} else if data.CommandTag.Update() {
		span.SetData("db.operation", "UPDATE")
	} else {
		span.SetData("db.operation", data.CommandTag.String())
	}

	if config := conn.Config(); config != nil {
		span.SetData("db.name", config.Database)
		span.SetData("server.address", config.Host)
		span.SetData("server.port", strconv.FormatUint(uint64(config.Port), 10))
	}

	if data.Err != nil {
		span.Status = sentry.SpanStatusInternalError
		span.SetData("error", data.Err.Error())
	}

	span.Finish()
}

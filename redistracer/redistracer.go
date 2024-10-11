// redistracer provides a hook that you can use with redis/go-redis package.
//
// Example usage:
//
//	package main
//
//	import "github.com/redis/go-redis/v9"
//	import "github.com/aldy505/sentry-integration/redistracer"
//
//	func main() {
//		redisOptions, err := redis.ParseURL(secrets.RedisAddress)
//		if err != nil {
//			slog.Error("failed to parse redis url", slog.String("error", err.Error()))
//			return
//		}
//
//		redisClient = redis.NewClient(redisOptions)
//
//		if err := redistracer.InstrumentTracing(redisClient); err != nil {
//			slog.Error("failed to instrument redis client", slog.String("error", err.Error()))
//		}
//	}
package redistracer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/redis/go-redis/extra/rediscmd/v9"
	"github.com/redis/go-redis/v9"
)

func InstrumentTracing(rdb redis.UniversalClient, opts ...TracingOption) error {
	switch rdb := rdb.(type) {
	case *redis.Client:
		opt := rdb.Options()
		connString := formatDBConnString(opt.Network, opt.Addr)
		opts = addServerAttributes(opts, opt.Addr)
		rdb.AddHook(newTracingHook(connString, opts...))
		return nil
	case *redis.ClusterClient:
		rdb.AddHook(newTracingHook("", opts...))
		rdb.OnNewNode(func(rdb *redis.Client) {
			opt := rdb.Options()
			opts = addServerAttributes(opts, opt.Addr)
			connString := formatDBConnString(opt.Network, opt.Addr)
			rdb.AddHook(newTracingHook(connString, opts...))
		})
		return nil
	case *redis.Ring:
		rdb.AddHook(newTracingHook("", opts...))
		rdb.OnNewNode(func(rdb *redis.Client) {
			opt := rdb.Options()
			opts = addServerAttributes(opts, opt.Addr)
			connString := formatDBConnString(opt.Network, opt.Addr)
			rdb.AddHook(newTracingHook(connString, opts...))
		})
		return nil
	default:
		return fmt.Errorf("redisotel: %T not supported", rdb)
	}
}

type tracingHook struct {
	conf *config
}

var _ redis.Hook = (*tracingHook)(nil)

func newTracingHook(connString string, opts ...TracingOption) *tracingHook {
	baseOpts := make([]baseOption, len(opts))
	for i, opt := range opts {
		baseOpts[i] = opt
	}
	conf := newConfig(baseOpts...)

	return &tracingHook{
		conf: conf,
	}
}

func (th *tracingHook) DialHook(hook redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		span := sentry.StartSpan(ctx, "redis.dial")
		ctx = span.Context()
		span.Data = th.conf.spanData
		defer span.Finish()

		conn, err := hook(ctx, network, addr)
		if err != nil {
			span.Status = sentry.SpanStatusInternalError
			return nil, err
		}

		span.Status = sentry.SpanStatusOK
		return conn, nil
	}
}

func (th *tracingHook) ProcessHook(hook redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		parentSpan := sentry.SpanFromContext(ctx)
		if parentSpan == nil {
			return hook(ctx, cmd)
		}
		ctx = parentSpan.Context()

		fn, file, line := funcFileLine("github.com/redis/go-redis")

		spanData := make(map[string]any)
		for key, value := range th.conf.spanData {
			spanData[key] = value
		}
		spanData["code.function"] = fn
		spanData["code.filepath"] = file
		spanData["code.lineno"] = line

		if th.conf.dbStmtEnabled {
			cmdString := rediscmd.CmdString(cmd)
			spanData["db.statement"] = cmdString
		}

		var spanOperation string
		switch strings.ToLower(cmd.Name()) {
		case "get", "mget", "hget", "hgetall", "hmget":
			spanOperation = "cache.get"
			break
		case "set", "setex", "mset", "hmset", "hset", "hsetnx", "lset", "msetnx", "psetex", "setbit", "setnx", "setrange":
			spanOperation = "cache.put"
			break
		case "del", "hdel":
			spanOperation = "cache.remove"
			break
		case "flushall", "flushdb":
			spanOperation = "cache.flush"
			break
		default:
			spanOperation = "db.redis"
		}

		span := parentSpan.StartChild(spanOperation, sentry.WithDescription(cmd.FullName()))
		ctx = span.Context()
		span.Data = spanData
		defer span.Finish()

		// Get key name if any
		if len(cmd.Args()) > 1 {
			span.SetData("cache.key", cmd.Args()[1])
		}

		if err := hook(ctx, cmd); err != nil {
			if errors.Is(err, redis.Nil) {
				span.SetData("cache.hit", false)
			}
			span.SetData("cache.success", false)
			span.Status = sentry.SpanStatusInternalError
			return err
		}

		span.SetData("cache.hit", true)
		span.SetData("cache.success", true)
		span.Status = sentry.SpanStatusOK
		return nil
	}
}

func (th *tracingHook) ProcessPipelineHook(hook redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		parentSpan := sentry.SpanFromContext(ctx)
		if parentSpan == nil {
			return hook(ctx, cmds)
		}
		ctx = parentSpan.Context()

		fn, file, line := funcFileLine("github.com/redis/go-redis")

		spanData := make(map[string]any)
		for key, value := range th.conf.spanData {
			spanData[key] = value
		}
		spanData["code.function"] = fn
		spanData["code.filepath"] = file
		spanData["code.lineno"] = line
		spanData["db.redis.num_cmd"] = len(cmds)

		summary, cmdsString := rediscmd.CmdsString(cmds)
		if th.conf.dbStmtEnabled {
			spanData["db.statement"] = cmdsString
		}

		span := parentSpan.StartChild("redis.pipeline "+summary, sentry.WithDescription(cmds[0].FullName()))
		ctx = span.Context()
		span.Data = th.conf.spanData
		defer span.Finish()

		if err := hook(ctx, cmds); err != nil {
			span.Status = sentry.SpanStatusInternalError
			return err
		}

		span.Status = sentry.SpanStatusOK
		return nil
	}
}

func formatDBConnString(network, addr string) string {
	if network == "tcp" {
		network = "redis"
	}
	return fmt.Sprintf("%s://%s", network, addr)
}

func funcFileLine(pkg string) (string, string, int) {
	const depth = 16
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	ff := runtime.CallersFrames(pcs[:n])

	var fn, file string
	var line int
	for {
		f, ok := ff.Next()
		if !ok {
			break
		}
		fn, file, line = f.Function, f.File, f.Line
		if !strings.Contains(fn, pkg) {
			break
		}
	}

	if ind := strings.LastIndexByte(fn, '/'); ind != -1 {
		fn = fn[ind+1:]
	}

	return fn, file, line
}

// Database span attributes semantic conventions recommended server address and port
// https://opentelemetry.io/docs/specs/semconv/database/database-spans/#connection-level-attributes
func addServerAttributes(opts []TracingOption, addr string) []TracingOption {
	host, portString, err := net.SplitHostPort(addr)
	if err != nil {
		return opts
	}

	// Parse the port string to an integer
	port, err := strconv.Atoi(portString)
	if err != nil {
		return opts
	}

	opts = append(opts, WithAttributes(map[string]any{
		"network.peer.address": host,
		"network.peer.port":    port,
	}))

	return opts
}

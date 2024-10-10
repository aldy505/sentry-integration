package redistracer

type config struct {
	// Common options.

	dbSystem string
	spanData map[string]any

	dbStmtEnabled bool

	poolName string
}

type baseOption interface {
	apply(conf *config)
}

type Option interface {
	baseOption
	tracing()
	metrics()
}

type option func(conf *config)

func (fn option) apply(conf *config) {
	fn(conf)
}

func (fn option) tracing() {}

func (fn option) metrics() {}

func newConfig(opts ...baseOption) *config {
	conf := &config{
		dbSystem: "redis",
		spanData: make(map[string]any),

		dbStmtEnabled: true,
	}

	for _, opt := range opts {
		opt.apply(conf)
	}

	conf.spanData["db.system"] = conf.dbSystem

	return conf
}

func WithDBSystem(dbSystem string) Option {
	return option(func(conf *config) {
		conf.dbSystem = dbSystem
	})
}

// WithAttributes specifies additional attributes to be added to the span.
func WithAttributes(attrs map[string]any) Option {
	return option(func(conf *config) {
		for k, v := range attrs {
			conf.spanData[k] = v
		}
	})
}

//------------------------------------------------------------------------------

type TracingOption interface {
	baseOption
	tracing()
}

type tracingOption func(conf *config)

var _ TracingOption = (*tracingOption)(nil)

func (fn tracingOption) apply(conf *config) {
	fn(conf)
}

func (fn tracingOption) tracing() {}

// WithDBStatement tells the tracing hook not to log raw redis commands.
func WithDBStatement(on bool) TracingOption {
	return tracingOption(func(conf *config) {
		conf.dbStmtEnabled = on
	})
}

package sqltracer

type SentrySqlTracerOption func(*sentrySqlConfig)

func WithDatabaseSystem(system string) SentrySqlTracerOption {
	return func(config *sentrySqlConfig) {
		config.databaseSystem = system
	}
}
func WithDatabaseName(name string) SentrySqlTracerOption {
	return func(config *sentrySqlConfig) {
		config.databaseName = name
	}
}

func WithServerAddress(address string, port string) SentrySqlTracerOption {
	return func(config *sentrySqlConfig) {
		config.serverAddress = address
		config.serverPort = port
	}
}

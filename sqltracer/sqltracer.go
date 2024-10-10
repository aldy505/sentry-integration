// sqltracer implements a sql driver that wraps the original driver and adds Sentry tracing capabilities
// to all queries. The trace is valid if there is a parent span exists in the context.
//
// Please note that when initializing the sqltracer, you will need to specify the `sqltracer.WithDatabaseSystem`
// option, so that Sentry will picks up the span as a correct query trace.
//
// Example with "github.com/go-sql-driver/mysql" package.
//
//	package main
//
//	import "github.com/go-sql-driver/mysql"
//	import "github.com/aldy505/sentry-integration/sqltracer"
//
//	func main() {
//		sql.Register("sentrymysql", sqltracer.NewSentrySql(&mysql.MySQLDriver{}, sqltracer.WithDatabaseSystem("mysql"), sqltracer.WithDatabaseName("mydb"), sqltracer.WithServerAddress("localhost", "3306")))
//
//		db, err := sql.Open("sentrymysql", "username:password@protocol(address)/dbname?param=value")
//	}
package sqltracer

type sentrySqlConfig struct {
	databaseSystem string
	databaseName   string
	serverAddress  string
	serverPort     string
}

func NewSentrySql(driver DriverConnector, options ...SentrySqlTracerOption) DriverConnector {
	var config sentrySqlConfig
	for _, option := range options {
		option(&config)
	}

	return &sentrySqlDriver{originalDriver: driver, config: &config}
}

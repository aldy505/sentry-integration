package sqltracer

import (
	"context"
	"database/sql/driver"
)

type DriverConnector interface {
	driver.Driver
	driver.DriverContext
}

type sentrySqlDriver struct {
	originalDriver DriverConnector
	config         *sentrySqlConfig
}

func (s *sentrySqlDriver) OpenConnector(name string) (driver.Connector, error) {
	connector, err := s.originalDriver.OpenConnector(name)
	if err != nil {
		return nil, err
	}

	return &sentrySqlConnector{originalConnector: connector, config: s.config}, nil
}

func (s *sentrySqlDriver) Open(name string) (driver.Conn, error) {
	conn, err := s.originalDriver.Open(name)
	if err != nil {
		return nil, err
	}

	return &sentryConn{originalConn: conn, config: s.config}, nil
}

type sentrySqlConnector struct {
	originalConnector driver.Connector
	config            *sentrySqlConfig
}

func (s *sentrySqlConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := s.originalConnector.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return &sentryConn{originalConn: conn, ctx: ctx, config: s.config}, nil
}

func (s *sentrySqlConnector) Driver() driver.Driver {
	return s.originalConnector.Driver()
}

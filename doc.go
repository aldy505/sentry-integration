// Package sentryintegration provides a set of drop-in replacement for some of popular Go packages
// to have it auto instrumented by Sentry dependency.
//
// Why not just use OpenTelemetry dependency? Well if you're already uses Sentry and not dependend
// by OpenTelemetry at all, and to fight the current issue that by using OpenTelemetry span processor
// the SDK will capture 100% of spans, whatever value you're setting on at the front initialization
// wouldn't be respected.
package sentryintegration

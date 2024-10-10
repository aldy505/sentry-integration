// Package httpclient provides a tracer implementation for net/http.RoundTripper.
//
//	roundTrippper := httpclient.NewSentryRoundTripper(nil, nil)
//	client := &http.Client{
//		Transport: roundTripper,
//	}
//
//	request, err := client.Do(request)
package httpclient

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
)

type SentryRoundTripTracerOption func(*SentryRoundTripper)

func WithTags(tags map[string]string) SentryRoundTripTracerOption {
	return func(t *SentryRoundTripper) {
		for k, v := range tags {
			t.tags[k] = v
		}
	}
}

func WithTag(key, value string) SentryRoundTripTracerOption {
	return func(t *SentryRoundTripper) {
		t.tags[key] = value
	}
}

func NewSentryRoundTripper(originalRoundTripper http.RoundTripper, tracePropagationTargets []string, opts ...SentryRoundTripTracerOption) http.RoundTripper {
	if originalRoundTripper == nil {
		originalRoundTripper = http.DefaultTransport
	}

	t := &SentryRoundTripper{
		originalRoundTripper:    originalRoundTripper,
		tracePropagationTargets: tracePropagationTargets,
		tags:                    make(map[string]string),
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

type SentryRoundTripper struct {
	originalRoundTripper    http.RoundTripper
	tracePropagationTargets []string

	tags map[string]string
}

func (s *SentryRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	// Start Sentry trace
	ctx := request.Context()
	parentSpan := sentry.SpanFromContext(ctx)
	if parentSpan == nil {
		return s.originalRoundTripper.RoundTrip(request)
	}
	ctx = parentSpan.Context()

	// Respect trace propagation targets
	if len(s.tracePropagationTargets) > 0 {
		requestUrlString := request.URL.String()
		for _, t := range s.tracePropagationTargets {
			if strings.Contains(requestUrlString, t) {
				continue
			}

			return s.originalRoundTripper.RoundTrip(request)
		}
	}

	cleanRequestURL := request.URL.Path

	span := sentry.StartSpan(ctx, "http.client", sentry.WithTransactionName(fmt.Sprintf("%s %s", request.Method, cleanRequestURL)))
	span.Tags = s.tags
	defer span.Finish()

	span.SetData("http.query", request.URL.Query().Encode())
	span.SetData("http.fragment", request.URL.Fragment)
	span.SetData("http.request.method", request.Method)

	request.Header.Add("Baggage", span.ToBaggage())
	request.Header.Add("Sentry-Trace", span.ToSentryTrace())

	response, err := s.originalRoundTripper.RoundTrip(request)

	if response != nil {
		span.Status = sentry.HTTPtoSpanStatus(response.StatusCode)
		span.SetData("http.response.status_code", response.Status)
		span.SetData("http.response_content_length", strconv.FormatInt(response.ContentLength, 10))
	}

	return response, err
}

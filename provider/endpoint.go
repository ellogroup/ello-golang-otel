package provider

import (
	"fmt"
	"net/url"
)

// resolveSignalEndpoint appends defaultPath to endpoint when endpoint has no path
// component. go.opentelemetry.io/otel v1.45.0 stopped defaulting a path-less
// WithEndpointURL to the signal path (/v1/traces, /v1/metrics) and now targets the
// literal root instead, which silently 404s against collectors that only listen on the
// signal paths (Aspire, the ADOT collector). We restore the old default here.
//
// A caller can still target the literal root explicitly by configuring the endpoint
// with a trailing "/" (e.g. "http://host:4318/"), since that gives the URL a non-empty
// path and is therefore left untouched.
func resolveSignalEndpoint(endpoint, defaultPath string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parsing OTLP endpoint %q: %w", endpoint, err)
	}
	if u.Path != "" {
		return endpoint, nil
	}
	return url.JoinPath(endpoint, defaultPath)
}

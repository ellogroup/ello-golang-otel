package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func setupTestMeter(t *testing.T) (*sdkmetric.ManualReader, metric.Meter) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	return reader, mp.Meter("test")
}

func collectMetrics(t *testing.T, reader *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	return rm
}

func findMetric(rm metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for _, sm := range rm.ScopeMetrics {
		for i := range sm.Metrics {
			if sm.Metrics[i].Name == name {
				return &sm.Metrics[i]
			}
		}
	}
	return nil
}

func testTracer() trace.Tracer {
	return sdktrace.NewTracerProvider().Tracer("test")
}

func TestNewAPIGatewayV1(t *testing.T) {
	tests := []struct {
		name           string
		handler        func(context.Context, events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)
		wantStatusCode int
	}{
		{
			name: "200 response, records counter and duration with status code attribute",
			handler: func(_ context.Context, _ events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
				return events.APIGatewayProxyResponse{StatusCode: 200}, nil
			},
			wantStatusCode: 200,
		},
		{
			name: "500 response, records counter and duration with 500 status code attribute",
			handler: func(_ context.Context, _ events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
				return events.APIGatewayProxyResponse{StatusCode: 500}, nil
			},
			wantStatusCode: 500,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, meter := setupTestMeter(t)
			mw := NewAPIGatewayV1(testTracer(), meter)
			h := mw.Wrap(tt.handler)

			_, err := h(context.Background(), events.APIGatewayProxyRequest{})
			require.NoError(t, err)

			rm := collectMetrics(t, reader)

			m := findMetric(rm, "faas.invocations")
			require.NotNilf(t, m, "faas.invocations metric not found")
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.Truef(t, ok, "faas.invocations data is not a Sum")
			require.Lenf(t, sum.DataPoints, 1, "faas.invocations data points")
			assert.Equalf(t, int64(1), sum.DataPoints[0].Value, "faas.invocations value")

			statusVal, ok := sum.DataPoints[0].Attributes.Value(attribute.Key("http.response.status_code"))
			assert.Truef(t, ok, "http.response.status_code attribute missing")
			assert.Equalf(t, int64(tt.wantStatusCode), statusVal.AsInt64(), "http.response.status_code")

			_, ok = sum.DataPoints[0].Attributes.Value(attribute.Key("faas.coldstart"))
			assert.Truef(t, ok, "faas.coldstart attribute missing")

			m = findMetric(rm, "faas.duration")
			require.NotNilf(t, m, "faas.duration metric not found")
			hist, ok := m.Data.(metricdata.Histogram[float64])
			require.Truef(t, ok, "faas.duration data is not a Histogram")
			require.Lenf(t, hist.DataPoints, 1, "faas.duration data points")
			assert.Equalf(t, uint64(1), hist.DataPoints[0].Count, "faas.duration count")
		})
	}
}

func TestNewAPIGatewayV1_ColdStartSequence(t *testing.T) {
	reader, meter := setupTestMeter(t)
	warmedUp.Store(false)
	t.Cleanup(func() { warmedUp.Store(true) })

	mw := NewAPIGatewayV1(testTracer(), meter)
	h := mw.Wrap(func(_ context.Context, _ events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	})

	_, err := h(context.Background(), events.APIGatewayProxyRequest{})
	require.NoError(t, err)
	_, err = h(context.Background(), events.APIGatewayProxyRequest{})
	require.NoError(t, err)

	rm := collectMetrics(t, reader)
	m := findMetric(rm, "faas.invocations")
	require.NotNil(t, m)

	sum, ok := m.Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Lenf(t, sum.DataPoints, 2, "expected two data points for cold and warm invocations")

	coldStarts := map[bool]int64{}
	for _, dp := range sum.DataPoints {
		val, ok := dp.Attributes.Value(attribute.Key("faas.coldstart"))
		require.True(t, ok)
		coldStarts[val.AsBool()] = dp.Value
	}
	assert.Equalf(t, int64(1), coldStarts[true], "cold start invocation count")
	assert.Equalf(t, int64(1), coldStarts[false], "warm invocation count")
}

func TestNewNoResponse(t *testing.T) {
	errHandler := errors.New("handler error")

	tests := []struct {
		name          string
		handler       func(context.Context, string) error
		wantErr       assert.ErrorAssertionFunc
		wantFaasError bool
	}{
		{
			name:          "successful handler, records counter with faas.error false",
			handler:       func(_ context.Context, _ string) error { return nil },
			wantErr:       assert.NoError,
			wantFaasError: false,
		},
		{
			name:          "handler returns error, records counter with faas.error true",
			handler:       func(_ context.Context, _ string) error { return errHandler },
			wantErr:       assert.Error,
			wantFaasError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, meter := setupTestMeter(t)
			mw := NewNoResponse[string](testTracer(), meter, "test-op")
			h := mw.Wrap(tt.handler)

			tt.wantErr(t, h(context.Background(), "payload"), "handler error")

			rm := collectMetrics(t, reader)

			m := findMetric(rm, "faas.invocations")
			require.NotNilf(t, m, "faas.invocations metric not found")
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.Truef(t, ok, "faas.invocations data is not a Sum")
			require.Lenf(t, sum.DataPoints, 1, "faas.invocations data points")
			assert.Equalf(t, int64(1), sum.DataPoints[0].Value, "faas.invocations value")

			errVal, ok := sum.DataPoints[0].Attributes.Value(attribute.Key("faas.error"))
			assert.Truef(t, ok, "faas.error attribute missing")
			assert.Equalf(t, tt.wantFaasError, errVal.AsBool(), "faas.error value")

			_, ok = sum.DataPoints[0].Attributes.Value(attribute.Key("faas.coldstart"))
			assert.Truef(t, ok, "faas.coldstart attribute missing")

			m = findMetric(rm, "faas.duration")
			require.NotNilf(t, m, "faas.duration metric not found")
			hist, ok := m.Data.(metricdata.Histogram[float64])
			require.Truef(t, ok, "faas.duration data is not a Histogram")
			require.Lenf(t, hist.DataPoints, 1, "faas.duration data points")
			assert.Equalf(t, uint64(1), hist.DataPoints[0].Count, "faas.duration count")
		})
	}
}

package handler_test

import (
	"context"
	"github.com/ellogroup/ello-golang-otel/slog/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"log/slog"
	"testing"
	"time"
)

type MockHandler struct {
	mock.Mock
}

func (m *MockHandler) Enabled(ctx context.Context, level slog.Level) bool {
	args := m.Called(ctx, level)
	return args.Bool(0)
}

func (m *MockHandler) Handle(ctx context.Context, r slog.Record) error {
	args := m.Called(ctx, r)
	return args.Error(0)
}

func (m *MockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	args := m.Called(attrs)
	return args.Get(0).(slog.Handler)
}

func (m *MockHandler) WithGroup(name string) slog.Handler {
	args := m.Called(name)
	return args.Get(0).(slog.Handler)
}

func TestHandle_WithValidSpan_AddsTraceAttributes(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	sc := span.SpanContext()
	expectedTraceID := sc.TraceID().String()
	expectedSpanID := sc.SpanID().String()

	m := new(MockHandler)
	m.On("Handle", mock.Anything, mock.MatchedBy(func(r slog.Record) bool {
		attrs := make(map[string]string)
		r.Attrs(func(a slog.Attr) bool {
			attrs[a.Key] = a.Value.String()
			return true
		})
		return attrs["trace_id"] == expectedTraceID && attrs["span_id"] == expectedSpanID
	})).Return(nil)

	h := handler.New(m)
	require.NoError(t, h.Handle(ctx, slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)))
	m.AssertExpectations(t)
}

func TestHandle_WithNoSpan_DoesNotAddTraceAttributes(t *testing.T) {
	m := new(MockHandler)
	m.On("Handle", mock.Anything, mock.MatchedBy(func(r slog.Record) bool {
		var hasTraceID, hasSpanID bool
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "trace_id" {
				hasTraceID = true
			}
			if a.Key == "span_id" {
				hasSpanID = true
			}
			return true
		})
		return !hasTraceID && !hasSpanID
	})).Return(nil)

	h := handler.New(m)
	require.NoError(t, h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)))
	m.AssertExpectations(t)
}

func TestEnabled_DelegatesToInner(t *testing.T) {
	m := new(MockHandler)
	m.On("Enabled", mock.Anything, slog.LevelInfo).Return(false)
	m.On("Enabled", mock.Anything, slog.LevelWarn).Return(true)

	h := handler.New(m)

	assert.False(t, h.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, h.Enabled(context.Background(), slog.LevelWarn))
	m.AssertExpectations(t)
}

func TestWithAttrs_ForwardsToInner(t *testing.T) {
	inner := new(MockHandler)
	m := new(MockHandler)
	attrs := []slog.Attr{slog.String("service", "test")}
	m.On("WithAttrs", attrs).Return(inner)

	h := handler.New(m)
	h2 := h.WithAttrs(attrs)

	assert.NotNil(t, h2)
	m.AssertExpectations(t)
}

func TestWithGroup_ForwardsToInner(t *testing.T) {
	inner := new(MockHandler)
	m := new(MockHandler)
	m.On("WithGroup", "mygroup").Return(inner)

	h := handler.New(m)
	h2 := h.WithGroup("mygroup")

	assert.NotNil(t, h2)
	m.AssertExpectations(t)
}

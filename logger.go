package main

import (
	"context"
	"log/slog"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type DatadogHandler struct {
	slog.Handler
}

func NewDatadogHandler(s slog.Handler) slog.Handler {
	return &DatadogHandler{
		s,
	}
}

func (h *DatadogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

func (h *DatadogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &DatadogHandler{h.WithAttrs(attrs)}
}

func (h *DatadogHandler) WithGroup(name string) slog.Handler {
	return &DatadogHandler{h.WithGroup(name)}
}

func (h *DatadogHandler) Handle(ctx context.Context, r slog.Record) error {
	span, ok := tracer.SpanFromContext(ctx)
	defer span.Finish()
	if ok {
		// spanがある場合は、spanの情報をlogに付与する
		traceId := span.Context().TraceID()
		spanId := span.Context().SpanID()
		group := slog.Group("dd",
			slog.Uint64("trace_id", traceId),
			slog.Uint64("span_id", spanId),
		)

		r.Add(group)
	}
	return h.Handler.Handle(ctx, r)
}

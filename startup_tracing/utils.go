package startup_tracing

import (
	"context"
	"github.com/modern-go/gls"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// a span that does nothing
var noopSpan = opentracing.NoopTracer{}.StartSpan("")

type activeSpan struct{}

var activeSpanKey = activeSpan{}

// Returns the current span, or nil, if no span is currently set
// in local storage.
//
// Deprecated: Start using the version with an explicit context parameter.
//
func CurrentSpan() opentracing.Span {
	if g := gls.GetGls(gls.GoID()); g != nil {
		if span, ok := g[activeSpanKey].(opentracing.Span); ok {
			return span
		}
	}

	return nil
}

// Runs the given function with the provided span
// set in local storage for the duration of function call.
// This method will not call 'Finish()' on the span
//
// Deprecated: Please propagate spans using contexts.
//
func WithSpan(span opentracing.Span, fn func()) {
	if g := gls.GetGls(gls.GoID()); g != nil {
		previousSpan := g[activeSpanKey]
		g[activeSpanKey] = span

		// restore previous span later
		defer func() {
			g[activeSpanKey] = previousSpan
		}()

	}

	fn()
}

// Runs an operation and traces it with the given name. This will create a
// new child span if some span is currently active.
//
// Deprecated: propagate spans using context.
//
func TraceChild(op string, fn func(span opentracing.Span) error) (err error) {
	return trace(op, false, fn)
}

// Runs an operation and traces it with the given name. This will create a
// new span if no span is currently active.
//
// Deprecated: propagate spans using context.
//
func TraceOrCreate(op string, fn func(span opentracing.Span) error) (err error) {
	return trace(op, true, fn)
}

func trace(op string, always bool, fn func(span opentracing.Span) error) (err error) {
	span := noopSpan

	if g := gls.GetGls(gls.GoID()); g != nil {
		previousSpan, ok := g[activeSpanKey].(opentracing.Span)

		if ok && previousSpan != nil {
			// build a child span
			span = previousSpan.Tracer().StartSpan(op,
				ext.SpanKindRPCClient,
				opentracing.ChildOf(previousSpan.Context()))
		} else if always {
			// start a new one
			span = opentracing.StartSpan(op, ext.SpanKindRPCClient)
		}

		g[activeSpanKey] = span

		defer func() {
			g[activeSpanKey] = previousSpan

			if err != nil {
				span.SetTag("error", true)
				span.SetTag("error_message", err.Error())
			}

			span.Finish()
		}()
	}

	err = fn(span)
	return
}

// Trace a child call while propagating the span using the context.
func TraceChildContext(ctx context.Context, op string, fn func(ctx context.Context, span opentracing.Span) error) (err error) {
	parentSpan := spanFromContextOrGLS(ctx)
	if parentSpan == nil {
		return fn(ctx, noopSpan)
	}

	span := parentSpan.Tracer().StartSpan(op,
		ext.SpanKindRPCClient,
		opentracing.ChildOf(parentSpan.Context()))

	defer func() {
		if err != nil {
			span.SetTag("error", true)
			span.SetTag("error_message", err.Error())
		}

		span.Finish()
	}()

	err = fn(opentracing.ContextWithSpan(ctx, span), span)
	return
}

// Returns the current span, or nil, if no span is currently set
// in local storage.
//
func spanFromContextOrGLS(ctx context.Context) opentracing.Span {
	span := opentracing.SpanFromContext(ctx)
	if span != nil {
		return span
	}

	if g := gls.GetGls(gls.GoID()); g != nil {
		if span, ok := g[activeSpanKey].(opentracing.Span); ok {
			return span
		}
	}

	return nil
}

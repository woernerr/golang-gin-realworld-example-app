package instrumentation

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func Setup(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	shutdown := func(context.Context) error { return nil }
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	if strings.EqualFold(os.Getenv("OTEL_SDK_DISABLED"), "true") ||
		os.Getenv("OTEL_TRACES_EXPORTER") == "none" ||
		(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "") {
		return shutdown, nil
	}

	res, err := resource.New(ctx,
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			attribute.String("service.version", "unknown"),
			attribute.String("deployment.environment.name", "unknown"),
			attribute.String("service.owner.url", "https://github.com/woernerr/golang-gin-realworld-example-app"),
		),
		resource.WithFromEnv(),
		resource.WithAttributes(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		return shutdown, err
	}
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return shutdown, err
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter), sdktrace.WithResource(res))
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}

func RouteNameMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		switch method {
		case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodDelete,
			http.MethodConnect, http.MethodOptions, http.MethodTrace, http.MethodPatch:
		default:
			method = "HTTP"
		}
		name := method
		if route := c.FullPath(); route != "" {
			name += " " + route
		}
		trace.SpanFromContext(c.Request.Context()).SetName(name)
		c.Next()
	}
}

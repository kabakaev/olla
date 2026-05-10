package middleware

import (
	"net"
	"net/http"

	"github.com/thushan/olla/internal/core/constants"
	"github.com/thushan/olla/internal/util"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

func TelemetryMiddleware(trustProxyHeaders bool, trustedProxyCIDRsParsed []*net.IPNet, skipHealthTraces bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		options := []otelhttp.Option{
			otelhttp.WithTracerProvider(otel.GetTracerProvider()),
		}
		if skipHealthTraces {
			options = append(options, otelhttp.WithFilter(func(r *http.Request) bool {
				return r.URL.Path != constants.DefaultHealthCheckEndpoint
			}))
		}

		return otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := util.GetClientIP(r, trustProxyHeaders, trustedProxyCIDRsParsed)
			requestID := GetRequestID(r.Context())
			span := trace.SpanFromContext(r.Context())
			if span != nil {
				attrs := []attribute.KeyValue{
					semconv.ClientAddress(clientIP),
					semconv.UserAgentOriginal(r.UserAgent()),
				}
				if requestID != "" {
					attrs = append(attrs, attribute.String("request.id", requestID))
				}
				if routePrefix, ok := r.Context().Value(constants.ContextRoutePrefixKey).(string); ok && routePrefix != "" {
					attrs = append(attrs, attribute.String("http.route", routePrefix))
				}
				span.SetAttributes(attrs...)
			}

			next.ServeHTTP(w, r)
		}), "http.server", options...)
	}
}

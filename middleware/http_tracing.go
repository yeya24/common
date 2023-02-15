package middleware

import (
	"fmt"
	"github.com/opentracing/opentracing-go"
	"net/http"

	"github.com/opentracing-contrib/go-stdlib/nethttp"
)

const (
	// ForceTracingHeader is a request header name that forces tracing sampling.
	ForceTracingHeader = "X-AMP-Force-Tracing"
)

// Dummy dependency to enforce that we have a nethttp version newer
// than the one which implements Websockets. (No semver on nethttp)
var _ = nethttp.MWURLTagFunc

// Tracer is a middleware which traces incoming requests.
type Tracer struct {
	RouteMatcher RouteMatcher
	SourceIPs    *SourceIPExtractor
}

// Wrap implements Interface
func (t Tracer) Wrap(next http.Handler) http.Handler {
	options := []nethttp.MWOption{
		nethttp.OperationNameFunc(func(r *http.Request) string {
			op := getRouteName(t.RouteMatcher, r)
			if op == "" {
				return "HTTP " + r.Method
			}

			return fmt.Sprintf("HTTP %s - %s", r.Method, op)
		}),
		nethttp.MWSpanObserver(func(sp opentracing.Span, r *http.Request) {
			// add a tag with the client's user agent to the span
			userAgent := r.Header.Get("User-Agent")
			if userAgent != "" {
				sp.SetTag("http.user_agent", userAgent)
			}

			// add a tag with the client's sourceIPs to the span, if a
			// SourceIPExtractor is given.
			if t.SourceIPs != nil {
				sp.SetTag("sourceIPs", t.SourceIPs.Get(r))
			}
		}),
		nethttp.MWStartSpanOptionsFunc(func(r *http.Request) []opentracing.StartSpanOption {
			var opts []opentracing.StartSpanOption
			val := r.Header.Get(ForceTracingHeader)
			if val != "" {
				opts = []opentracing.StartSpanOption{
					opentracing.Tag{Key: "thanos.force_tracing", Value: "true"},
				}
			}
			return opts
		}),
	}

	return nethttp.Middleware(opentracing.GlobalTracer(), next, options...)
}

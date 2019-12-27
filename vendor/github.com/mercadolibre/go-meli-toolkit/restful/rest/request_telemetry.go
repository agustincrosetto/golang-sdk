package rest

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/mercadolibre/go-meli-toolkit/godog"
)

// metricsCtxKey is used for decorating a context.Context with a rest.MetricsReportConfig
type metricsCtxKey struct{}

func contextWithMetricsConfig(ctx context.Context, config MetricsReportConfig) context.Context {
	return context.WithValue(ctx, metricsCtxKey{}, config)
}

func metricsConfigFromContext(ctx context.Context) MetricsReportConfig {
	id, _ := ctx.Value(metricsCtxKey{}).(MetricsReportConfig)
	return id
}

type tracedRoundTripper struct{ Transport http.RoundTripper }

func (t *tracedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	config := metricsConfigFromContext(ctx)

	// Fast path: metrics are disabled, just execute the given
	// request using the underlying transport.
	if config.DisableHttpConnectionsMetrics {
		return t.Transport.RoundTrip(req)
	}

	// Caller wants metrics for the given request, in order to provide them we
	// must create the different httptrace.ClientTrace contexts.
	req = req.WithContext(httptrace.WithClientTrace(ctx, t.newClientTrace(config.TargetId)))

	return t.Transport.RoundTrip(req)
}

func (t *tracedRoundTripper) newClientTrace(targetID string) *httptrace.ClientTrace {
	var (
		started           time.Time
		dnsStarTime       time.Time
		tlsHandshakeStart time.Time
		tcpConnectStart   time.Time
	)

	// Once this function exits we set the start time, many of the following
	// metrics record as value a delta of the start time and the time when
	// the given callback was called by the underlying transport.
	defer func() { started = time.Now() }()

	tags := []string{
		"target_id:" + targetID,
		"technology:go",
	}

	return &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			godog.RecordSimpleMetric("conn_request", 1, tags[0])
		},
		GotConn: func(connInfo httptrace.GotConnInfo) {
			if connInfo.Reused {
				godog.RecordSimpleMetric("conn_got", 1, "status:reused", tags[0])
			} else {
				godog.RecordSimpleMetric("conn_got", 1, "status:not_reused", tags[0])
			}
		},
		PutIdleConn: func(err error) {
			if err != nil {
				godog.RecordSimpleMetric("conn_put_idle", 1, "status:fail", tags[0])
			} else {
				godog.RecordSimpleMetric("conn_put_idle", 1, "status:ok", tags[0])
			}
		},

		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStarTime = time.Now()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			if !dnsStarTime.IsZero() {
				godog.RecordCompoundMetric("toolkit.http.dns.time", normalizeDuration(time.Since(dnsStarTime)), tags...)
			}
		},

		ConnectStart: func(network, addr string) {
			tcpConnectStart = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			if !tcpConnectStart.IsZero() {
				godog.RecordCompoundMetric("toolkit.http.tcp_connect.time", normalizeDuration(time.Since(tcpConnectStart)), tags...)
			}

			if err != nil {
				godog.RecordSimpleMetric("conn_new", 1, "status:fail", tags[0])
			} else {
				godog.RecordSimpleMetric("conn_new", 1, "status:ok", tags[0])
			}
		},

		TLSHandshakeStart: func() {
			tlsHandshakeStart = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, e error) {
			if !tlsHandshakeStart.IsZero() {
				godog.RecordCompoundMetric("toolkit.http.tls_handshake.time", normalizeDuration(time.Since(tlsHandshakeStart)), tags...)
			}
		},

		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if !started.IsZero() {
				godog.RecordCompoundMetric("toolkit.http.request_written.time", normalizeDuration(time.Since(started)), tags...)
			}
		},

		GotFirstResponseByte: func() {
			if !started.IsZero() {
				godog.RecordCompoundMetric("toolkit.http.response_first_byte.time", normalizeDuration(time.Since(started)), tags...)
			}
		},
	}
}

func normalizeDuration(d time.Duration) float64 {
	return d.Seconds() * 1000.0
}

package rest

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mercadolibre/go-meli-toolkit/godog"
	"github.com/mercadolibre/go-meli-toolkit/tracing"
)

var readVerbs = [3]string{http.MethodGet, http.MethodHead, http.MethodOptions}
var contentVerbs = [3]string{http.MethodPost, http.MethodPut, http.MethodPatch}
var defaultCheckRedirectFunc func(req *http.Request, via []*http.Request) error

var maxAge = regexp.MustCompile(`(?:max-age|s-maxage)=(\d+)`)
var poolMap sync.Map

const httpDateFormat = "Mon, 01 Jan 2006 15:04:05 GMT"
const RETRY_HEADER = "X-Retry"

func (rb *RequestBuilder) DoRequest(verb string, reqURL string, reqBody interface{}, opts ...Option) *Response {
	var after func(bool)
	var err error

	var reqOpt reqOptions

	for _, opt := range opts {
		opt(&reqOpt)
	}

	if rb.circuitBreaker != nil {
		after, err = rb.circuitBreaker.Allow()
		if err != nil {
			return &Response{
				Err: err,
				Response: &http.Response{
					StatusCode: http.StatusInternalServerError,
				},
			}
		}
	}
	result := rb.doRequest(verb, reqURL, reqBody, reqOpt)

	if after != nil {
		success := result.Err == nil && result.StatusCode/100 != 5
		after(success)
	}

	return result
}

func (rb *RequestBuilder) doRequest(verb string, reqURL string, reqBody interface{}, opt reqOptions) (result *Response) {

	var cacheURL string
	var cacheResp *Response
	result = new(Response)
	reqURL = rb.BaseURL + reqURL

	// If Cache enable && operation is read: Cache GET
	if rb.EnableCache && matchVerbs(verb, readVerbs) {
		if cacheResp = resourceCache.get(reqURL); cacheResp != nil {
			cacheResp.cacheHit.Store(true)
			if !cacheResp.revalidate {
				return cacheResp
			}
		}
	}

	if !rb.EnableCache {
		rb.headersMtx.Lock()
		delete(rb.Headers, "If-None-Match")
		delete(rb.Headers, "If-Modified-Since")
		rb.headersMtx.Unlock()
	}

	// Marshal request to JSON or XML
	body, err := rb.marshalReqBody(reqBody)
	if err != nil {
		result.Err = err
		return
	}

	// Change URL to point to Mockup server
	reqURL, cacheURL, err = checkMockup(reqURL)
	if err != nil {
		result.Err = err
		return
	}

	// Make the request and
	var httpResp *http.Response
	var responseErr error

	end := false
	retries := 0
	for !end {
		request, err := http.NewRequest(verb, reqURL, bytes.NewBuffer(body))
		if err != nil {
			result.Err = err
			return
		}

		// Set extra parameters
		rb.setParams(request, cacheResp, cacheURL)

		// Decorate the request context with the builders MetricsConfig, this can then
		// be used by tracing functions to tag metrics.
		ctx := contextWithMetricsConfig(opt.Context(), rb.MetricsConfig)
		request = request.WithContext(ctx)

		if rb.poolName == "" {
			rb.poolName = getCallerFile()
			sendRestMetrics(rb)
		}

		request.Header.Set(socketTimeoutConfig, millisString(rb.getRequestTimeout()))
		request.Header.Set(restClientPoolName, rb.poolName)

		// Copy headers from options struct into new request object.
		headers := opt.Headers()
		for k := range headers {
			request.Header.Add(k, headers.Get(k))
		}

		// Copy tracing headers from request context.
		traceHeaders := tracing.ForwardedHeaders(request.Context())
		for header := range traceHeaders {
			value := traceHeaders.Get(header)

			// If the header was already added by the caller, and it's different from the
			// one we should be forwarding, instead of replacing it record a metric.
			if v := request.Header.Get(header); v != "" && v != value {
				godog.RecordSimpleMetric("platform.traffic.forwarded_header.diff", 1, new(godog.Tags).Add("stack", "go-meli-toolkit").Add("header", strings.ToLower(header)).ToArray()...)
				continue
			}

			request.Header.Set(header, value)
		}

		initTime := time.Now()

		httpResp, responseErr = rb.getClient().Do(request)
		if !rb.MetricsConfig.DisableApiCallMetrics {
			if responseErr != nil {
				godog.RecordApiCallMetric(rb.MetricsConfig.TargetId, initTime, "error", retries > 0)
			} else {
				godog.RecordApiCallMetric(rb.MetricsConfig.TargetId, initTime, strconv.Itoa(httpResp.StatusCode), retries > 0)
			}
		}

		if rb.RetryStrategy != nil {
			retryResp := rb.RetryStrategy.ShouldRetry(request, httpResp, responseErr, retries)
			if retryResp.Retry() {
				retryFunc := func() (interface{}, error) {
					// We might be retrying because of an error in the request. As stated
					// in https://godoc.org/net/http#Client.Do If the returned error
					// is nil, the Response will contain a non-nil Body which the
					// user is expected to close.
					if responseErr == nil {
						drainBody(httpResp.Body)
					}

					time.Sleep(retryResp.Delay())
					retries++
					request.Header.Set(RETRY_HEADER, strconv.Itoa(retries))
					return nil, nil
				}

				if rb.circuitBreaker != nil {
					_, _ = retryFunc()
					continue
				} else {
					if _, err := retryLimiter.Action(1, retryFunc); err == nil {
						continue
					}
				}
				if !rb.MetricsConfig.DisableApiCallMetrics {
					godog.RecordSimpleMetric("go.api_call.retry_break", 1, new(godog.Tags).Add("target_id", rb.MetricsConfig.TargetId).ToArray()...)
				}
			}
		}
		end = true
	}
	if responseErr != nil {
		result.Err = responseErr
		return
	}

	// Read response
	defer httpResp.Body.Close()
	respBody, err := ioutil.ReadAll(httpResp.Body)

	if err != nil {
		result.Err = err
		return
	}

	// If we get a 304, return response from cache
	if rb.EnableCache && (httpResp.StatusCode == http.StatusNotModified) {
		result = cacheResp
		return
	}

	result.Response = httpResp
	if !rb.UncompressResponse {
		result.byteBody = respBody
	} else {
		respEncoding := httpResp.Header.Get("Content-Encoding")
		if respEncoding == "" {
			respEncoding = httpResp.Header.Get("Content-Type")
		}
		switch respEncoding {
		case "gzip":
			fallthrough
		case "application/x-gzip":
			{
				if len(respBody) == 0 {
					break
				}
				gr, err := gzip.NewReader(bytes.NewBuffer(respBody))
				defer gr.Close()
				if err != nil {
					result.Err = err
				} else {
					uncompressedData, err := ioutil.ReadAll(gr)
					if err != nil {
						result.Err = err
					} else {
						result.byteBody = uncompressedData
					}
				}
			}
		default:
			{
				result.byteBody = respBody
			}
		}
	}

	ttl := setTTL(result)
	lastModified := setLastModified(result)
	etag := setETag(result)

	if !ttl && (lastModified || etag) {
		result.revalidate = true
	}

	// If Cache enable: Cache SETNX
	if rb.EnableCache && matchVerbs(verb, readVerbs) && (ttl || lastModified || etag) {
		resourceCache.setNX(cacheURL, result)
	}

	return
}

func checkMockup(reqURL string) (string, string, error) {

	cacheURL := reqURL

	if mockUpEnv {

		rURL, err := url.Parse(reqURL)
		if err != nil {
			return reqURL, cacheURL, err
		}

		rURL.Scheme = mockServerURL.Scheme
		rURL.Host = mockServerURL.Host

		return rURL.String(), cacheURL, nil
	}

	return reqURL, cacheURL, nil
}

func (rb *RequestBuilder) marshalReqBody(body interface{}) (b []byte, err error) {

	if body != nil {
		switch rb.ContentType {
		case JSON:
			b, err = json.Marshal(body)
		case XML:
			b, err = xml.Marshal(body)
		case BYTES:
			var ok bool
			b, ok = body.([]byte)
			if !ok {
				err = fmt.Errorf("bytes: body is %T(%v) not a byte slice", body, body)
			}
		}
	}

	return
}

func (rb *RequestBuilder) getClient() *http.Client {

	// This will be executed only once
	// per request builder
	rb.clientMtxOnce.Do(func() {

		rb.Client = &http.Client{
			Transport: &tracedRoundTripper{
				Transport: rb.getTransport(),
			},
			Timeout: rb.getConnectionTimeout() + rb.getRequestTimeout(),
		}

		if !rb.FollowRedirect {
			rb.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return errors.New("Avoided redirect attempt")
			}
		} else {
			rb.Client.CheckRedirect = defaultCheckRedirectFunc
		}
	})
	return rb.Client
}

func (rb *RequestBuilder) getTransport() http.RoundTripper {
	cp := rb.CustomPool
	if cp == nil {
		return rb.makeTransport()
	}

	cp.once.Do(func() {
		if cp.Transport == nil {
			cp.Transport = rb.makeTransport()
		} else if ctr, ok := cp.Transport.(*http.Transport); ok {
			ctr.DialContext = (&net.Dialer{Timeout: rb.getConnectionTimeout()}).DialContext
			ctr.ResponseHeaderTimeout = rb.getRequestTimeout()
		}
	})

	return cp.Transport
}

func (rb *RequestBuilder) makeTransport() http.RoundTripper {
	return &http.Transport{
		MaxIdleConnsPerHost:   rb.getMaxIdleConnsPerHost(),
		Proxy:                 rb.getProxy(),
		DialContext:           (&net.Dialer{Timeout: rb.getConnectionTimeout()}).DialContext,
		ResponseHeaderTimeout: rb.getRequestTimeout(),
	}
}

func (rb *RequestBuilder) getRequestTimeout() time.Duration {

	switch {
	case rb.DisableTimeout:
		return 0
	case rb.Timeout > 0:
		return rb.Timeout
	default:
		return DefaultTimeout
	}
}

func (rb *RequestBuilder) getConnectionTimeout() time.Duration {

	switch {
	case rb.DisableTimeout:
		return 0
	case rb.ConnectTimeout > 0:
		return rb.ConnectTimeout
	default:
		return DefaultConnectTimeout
	}
}

func (rb *RequestBuilder) getMaxIdleConnsPerHost() int {
	if cp := rb.CustomPool; cp != nil {
		return cp.MaxIdleConnsPerHost
	}
	return DefaultMaxIdleConnsPerHost
}

func (rb *RequestBuilder) getProxy() func(*http.Request) (*url.URL, error) {
	if cp := rb.CustomPool; cp != nil && cp.Proxy != "" {
		if proxy, err := url.Parse(cp.Proxy); err == nil {
			return http.ProxyURL(proxy)
		}
	}
	return http.ProxyFromEnvironment
}

func (rb *RequestBuilder) setParams(req *http.Request, cacheResp *Response, cacheURL string) {
	// Custom Headers
	if rb.Headers != nil && len(rb.Headers) > 0 {
		rb.headersMtx.RLock()
		for key, values := range map[string][]string(rb.Headers) {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
		rb.headersMtx.RUnlock()
	}

	// Default headers
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")

	// If mockup
	if mockUpEnv {
		req.Header.Set("X-Original-URL", cacheURL)
	}

	// Basic Auth
	if rb.BasicAuth != nil {
		req.SetBasicAuth(rb.BasicAuth.UserName, rb.BasicAuth.Password)
	}

	// User Agent
	req.Header.Set("User-Agent", func() string {
		if rb.UserAgent != "" {
			return rb.UserAgent
		}
		return "github.com/go-loco/restful"
	}())

	// Encoding
	var cType string

	switch rb.ContentType {
	case JSON:
		cType = "json"
	case XML:
		cType = "xml"
	}

	if cType != "" {
		req.Header.Set("Accept", "application/"+cType)

		if matchVerbs(req.Method, contentVerbs) {
			req.Header.Set("Content-Type", "application/"+cType)
		}
	}

	if cacheResp != nil && cacheResp.revalidate {
		switch {
		case cacheResp.etag != "":
			req.Header.Set("If-None-Match", cacheResp.etag)
		case cacheResp.lastModified != nil:
			req.Header.Set("If-Modified-Since", cacheResp.lastModified.Format(httpDateFormat))
		}
	}

}

func matchVerbs(s string, sarray [3]string) bool {
	for i := 0; i < len(sarray); i++ {
		if sarray[i] == s {
			return true
		}
	}

	return false
}

func setTTL(resp *Response) (set bool) {

	now := time.Now()

	// Cache-Control Header
	cacheControl := maxAge.FindStringSubmatch(resp.Header.Get("Cache-Control"))

	if len(cacheControl) > 1 {

		ttl, err := strconv.Atoi(cacheControl[1])
		if err != nil {
			return
		}

		if ttl > 0 {
			t := now.Add(time.Duration(ttl) * time.Second)
			resp.ttl = &t
			set = true
		}

		return
	}

	//Expires Header
	//Date format from RFC-2616, Section 14.21
	expires, err := time.Parse(httpDateFormat, resp.Header.Get("Expires"))
	if err != nil {
		return
	}

	if expires.Sub(now) > 0 {
		resp.ttl = &expires
		set = true
	}

	return
}

func setLastModified(resp *Response) bool {
	lastModified, err := time.Parse(httpDateFormat, resp.Header.Get("Last-Modified"))
	if err != nil {
		return false
	}

	resp.lastModified = &lastModified
	return true
}

func setETag(resp *Response) bool {

	resp.etag = resp.Header.Get("ETag")

	return resp.etag != ""
}

// We need to consume response bodies to maintain http connections, but
// limit the size we consume to respReadLimit.
const respReadLimit = int64(4096)

// Read & discard the given body until respReadLimit and close it.
//
// When a response body is given, closing it after EOF is reached
// means we can reuse the TCP connection.
//
// If the response body is bigger than respReadLimit then we give up and
// close the body, resulting in the connection being closed as well.
func drainBody(body io.ReadCloser) {
	defer body.Close()
	io.Copy(ioutil.Discard, io.LimitReader(body, respReadLimit))
}

func millisString(d time.Duration) string {
	return strconv.Itoa(int(d.Seconds() * 1000))
}

func sendRestMetrics(rb *RequestBuilder) {
	_, ok := poolMap.LoadOrStore(rb.poolName, true)
	if !ok {
		buildMetricData(rb)
	}
}

func getCallerFile() string {
	frame3 := getFrame(3)
	frame4 := getFrame(4)
	frame5 := getFrame(5)

	switch {
	case path.Base(frame4.File) == "rest.go":
		return "pool_" + strings.TrimSuffix(path.Base(frame5.File), ".go")
	case path.Base(frame3.File) == "requestbuilder.go":
		return "pool_" + strings.TrimSuffix(path.Base(frame4.File), ".go")
	case path.Base(frame3.File) == "concurrent.go":
		return "pool_concurrent_unknown"
	}

	return "pool_unknown"
}

func getFrame(skipFrames int) runtime.Frame {
	// We need the frame at index skipFrames+2, since we never want runtime.Callers and getFrame
	targetFrameIndex := skipFrames + 2

	// Set size to targetFrameIndex+2 to ensure we have room for one more caller than we need
	programCounters := make([]uintptr, targetFrameIndex+2)
	n := runtime.Callers(0, programCounters)

	frame := runtime.Frame{Function: "unknown"}
	if n > 0 {
		frames := runtime.CallersFrames(programCounters[:n])
		for more, frameIndex := true, 0; more && frameIndex <= targetFrameIndex; frameIndex++ {
			var frameCandidate runtime.Frame
			frameCandidate, more = frames.Next()
			if frameIndex == targetFrameIndex {
				frame = frameCandidate
			}
		}
	}
	return frame
}

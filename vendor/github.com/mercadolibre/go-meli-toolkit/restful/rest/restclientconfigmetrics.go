package rest

import (
	"os"
	"reflect"

	"github.com/mercadolibre/go-meli-toolkit/restful/rest/retry"
	"github.com/mercadolibre/go-meli-toolkit/tracing/newrelic"
)

const (
	applicationName     = "Application-Name"
	departmentName      = "Department-Name"
	scopeName           = "Scope-Name"
	deployVersion       = "Deploy-version"
	restClientPoolName  = "X-Rest-Pool-Name"
	socketTimeoutConfig = "X-Socket-Timeout"
	retryStrategyName   = "X-Retry-Strategy-Name"

	// simple
	maxRetriesAttemptParameter = "Max-Retries-Param"
	retryDelayParameter        = "Retry-Delay-Param"

	// exponential--> backoffRetryStrategy
	minWaitTimeParameter = "Min-Wait-Time-Param"
	maxWaitTimeParameter = "Max-Wait-Time-Param"
)

func populateRetryStrategyParams(strategy retry.RetryStrategy, eventAttributes map[string]interface{}) {
	// If the strategy does not implement a GetParams method then we
	// can't extract how it was configured.
	s, ok := strategy.(interface{ GetParams() map[string]interface{} })
	if !ok {
		eventAttributes[retryStrategyName] = "customRetryStrategy"
		return
	}

	param := s.GetParams()

	if reflect.TypeOf(strategy).Name() == "simpleRetryStrategy" {
		eventAttributes[retryStrategyName] = "simpleRetryStrategy"
		eventAttributes[maxRetriesAttemptParameter] = param["max_retries"]
		eventAttributes[retryDelayParameter] = param["delay"]
	} else {
		eventAttributes[retryStrategyName] = "backoffRetryStrategy"
		eventAttributes[minWaitTimeParameter] = param["min_wait"]
		eventAttributes[maxWaitTimeParameter] = param["max_wait"]
	}
}

func buildMetricData(rb *RequestBuilder) {
	eventAttributes := map[string]interface{}{
		applicationName:     os.Getenv("APPLICATION"),
		departmentName:      os.Getenv("DEPT"),
		scopeName:           os.Getenv("SCOPE"),
		deployVersion:       os.Getenv("VERSION"),
		restClientPoolName:  rb.poolName,
		socketTimeoutConfig: millisString(rb.getRequestTimeout()),
	}

	if rb.RetryStrategy != nil {
		populateRetryStrategyParams(rb.RetryStrategy, eventAttributes)
	}

	sendMetricsToInsights(eventAttributes)
}

// Send map metrics to a custom insight table
func sendMetricsToInsights(eventAttributes map[string]interface{}) {
	if len(eventAttributes) > 0 {
		app := newrelic.App()
		if app != nil {
			_ = app.RecordCustomEvent("RestClientApplicationConfigs", eventAttributes)
		}
	}
}

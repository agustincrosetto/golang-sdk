package newrelic

import (
	"fmt"
	"os"
	"sync"

	"github.com/mercadolibre/go-meli-toolkit/goutils/logger"
	newrelic "github.com/newrelic/go-agent"
)

const (
	newrelicKey = "9a84196d7cc629ddd9fb15d2ca99a2f3eb12048f"
)

var (
	app     newrelic.Application
	appOnce sync.Once
)

// We must do initialization at init() because NewRelic's agent has to "warm up"
// once it's instantiated and before it can start acquiring metrics.
func Init(ignoreStatus ...int) newrelic.Application {
	appOnce.Do(func() {
		config := newrelic.NewConfig(fmt.Sprintf("%s.%s", os.Getenv("SCOPE"), os.Getenv("APPLICATION")), newrelicKey)
		config.ErrorCollector.IgnoreStatusCodes = append(config.ErrorCollector.IgnoreStatusCodes, ignoreStatus...)

		application, err := newrelic.NewApplication(config)
		if err != nil {
			logger.Error("Could not create newrelic agent.", err)
			return
		}

		app = application
	})

	return app
}

// App returns a instantiated newrelic.Application
func App() newrelic.Application {
	return app
}

/**
 * @author mlabarinas, fersca
 */

package goutils

import (
	"fmt"
	json "github.com/json-iterator/go"
	"runtime/debug"
	"strings"
	"time"

	"github.com/mercadolibre/go-meli-toolkit/goutils/logger"
)

func ToJSONString(value interface{}) (string, error) {
	bytes, error := json.Marshal(value)

	return string(bytes), error
}

func ToJSON(value string) (interface{}, error) {
	var jsonResult interface{}

	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()

	if error := decoder.Decode(&jsonResult); error != nil {
		return nil, error

	} else {
		return jsonResult, nil
	}
}

func FromJSONTo(value string, instance interface{}) error {
	return json.Unmarshal([]byte(value), instance)
}

func Retry(fn func() error, times int, sleepDuration time.Duration) (err error) {
	for err = fn(); err != nil && times > 1; times, err = times-1, fn() {
		time.Sleep(sleepDuration)
	}
	return err
}

// Recover helps regains control of a panicking goroutine.
// Is only useful if used as the only function in a defer statement:
//		defer goutils.Recover()
// Never use it this way:
//		defer func() {
//			goutils.Recover()
//			...
//		}
// Because it would not handle panics in your goroutine, it would only handle panics in the anonymous defer function.
// For more info:
//		https://www.airs.com/blog/archives/376
//		https://groups.google.com/forum/#!msg/golang-nuts/SwmjC_j5q90/99rdN1LEN1kJ
func Recover() {
	if err := recover(); err != nil {
		logger.Errorf("[Custom Recovery] panic recovered: %s %s", fmt.Errorf("%s", err), debug.Stack(), err)
	}
}

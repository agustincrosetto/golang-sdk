# Goutils

Utils for Meli projects

  * [ApiErrors](#apierrors)
  * [Logger](#logger)

## ApiErrors

This package handles the creation of errors, following the standard used in most of the APIs in meli

For that end, the following interfaces are exposed:

```go
type CauseList []interface{}

type ApiError interface {
	Message() string
	Code() string
	Status() int
	Cause() CauseList
	Error() string
}
```

And an internal implementation that follows the json convention:

```go
type apiErr struct {
	ErrorMessage string    `json:"message"`
	ErrorCode    string    `json:"error"`
	ErrorStatus  int       `json:"status"`
	ErrorCause   CauseList `json:"cause"`
}
```

### Basic Usage

#### Import package

```go
import "github.com/mercadolibre/go-meli-toolkit/goutils/apierrors"
```

#### Standard error helpers
```go
func NewApiError(message string, error string, status int, cause CauseList) ApiError 

func NewNotFoundApiError(message string) ApiError 

func NewTooManyRequestsError(message string) ApiError 

func NewBadRequestApiError(message string) ApiError 

func NewValidationApiError(message string, error string, cause CauseList) ApiError 

func NewMethodNotAllowedApiError() ApiError 

func NewInternalServerApiError(message string, err error) 

func NewForbiddenApiError(message string) ApiError 

func NewUnauthorizedApiError(message string) ApiError 

func NewConflictApiError(id string) ApiError

func NewApiErrorFromBytes(data []byte) (ApiError, error)
```

## Logger

This package provides a standard logger based on [logrus](https://github.com/sirupsen/logrus)

### Basic Usage

#### Import package
```go
import "github.com/mercadolibre/go-meli-toolkit/goutils/logger"
```

The init() method will initialize the Log (```*logrus.Logger```) property. If needed, you can load this for side effects only:
```go
import _ "github.com/mercadolibre/go-meli-toolkit/goutils/logger"
```

#### Configuration

Set log severity level. This must be a valid [logrus level](https://github.com/sirupsen/logrus/blob/d682213848ed68c0a260ca37d6dd5ace8423f5ba/logrus.go#L38-L49) 
```go
logger.SetLogLevel("error")
```

Get output writer
```go
output := logger.GetOut()
```

#### Logging

Available methods
```go
func Print(e interface{})

func Debug(message string, tags ...string)

func Info(message string, tags ...string)

func Warn(message string, tags ...string)

func Error(message string, err error, tags ...string)

func Panic(message string, err error, tags ...string)

func Debugf(format string, args ...interface{})

func Infof(format string, args ...interface{})

func Warnf(format string, args ...interface{})

func Errorf(format string, err error, args ...interface{})

func Panicf(format string, err error, args ...interface{})
```

## Helpers
```go
func ToJSONString(value interface{}) (string, error)

func ToJSON(value string) (interface{}, error) 

func FromJSONTo(value string, instance interface{}) error 

func Retry(fn func() error, times int, sleepDuration time.Duration) (err error) 
```

## Questions?

[fury@mercadolibre.com](fury@mercadolibre.com)
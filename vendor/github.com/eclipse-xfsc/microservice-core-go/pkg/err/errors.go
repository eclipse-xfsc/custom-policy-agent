// Package errors defines structured errors which can
// be used for nesting errors with propagation
// of error identifiers and their messages.
// It also supports JSON serialization, so service to
// service communication can preserve error Kind.
package err

import (
	"bytes"
	"encoding/json"
	"net/http"
)

var separator = ": "

type Kind int

const (
	Unknown      Kind = iota // Unknown error.
	BadRequest               // BadRequest specifies invalid arguments or operation.
	Unauthorized             // Unauthorized request.
	Forbidden                // Forbidden operation.
	Exist                    // Exist already.
	NotFound                 // NotFound specifies that a resource does not exist.
	Timeout                  // Timeout of request.
	Internal                 // Internal error or inconsistency.
	ServiceUnavailable
)

type Error struct {
	// ID is a unique error identifier.
	ID string

	// Kind of error returned to the caller.
	Kind Kind

	// Message is a description of the error.
	Message string

	// The underlying error that triggered this one, if any.
	Err error
}

func (k Kind) String() string {
	switch k {
	case Unknown:
		return "unknown error"
	case BadRequest:
		return "bad request"
	case Unauthorized:
		return "not authenticated"
	case Forbidden:
		return "permission denied"
	case Exist:
		return "already exist"
	case NotFound:
		return "not found"
	case Timeout:
		return "timeout"
	case Internal:
		return "internal error"
	case ServiceUnavailable:
		return "service unavailable"
	}

	return "unknown error kind"
}

// New builds an error value from its arguments.
// There must be at least one argument or New panics.
// The type of each argument determines its meaning.
// If more than one argument of a given type is presented, only the last one is
// recorded.
//
// The supported types are:
//
//	errors.Kind:
//	    The kind of the error.
//	*errors.Error
//	    The underlying error that triggered this one. If the error has
//	    non-empty ID and Kind fields, they are promoted as values of the
//	    returned one.
//	error:
//	    The underlying error that triggered this one.
//	string:
//	    Treated as an error message and assigned to the Message field.
func New(args ...interface{}) error {
	if len(args) == 0 {
		panic("call to errors.New without arguments")
	}

	e := &Error{}
	var innerKind = Unknown
	for _, arg := range args {
		switch arg := arg.(type) {
		case Kind:
			e.Kind = arg
		case *Error:
			errCopy := *arg
			e.Err = &errCopy
			e.ID = errCopy.ID
			innerKind = errCopy.Kind
			if e.Message == "" {
				e.Message = errCopy.Message
			}
		case error:
			e.Err = arg
		case string:
			e.Message = arg
		}
	}

	if e.ID == "" {
		e.ID = NewID()
	}

	if e.Kind == Unknown {
		e.Kind = innerKind
	}

	return e
}

// Is reports whether err is an *Error of the given Kind.
func Is(kind Kind, err error) bool {
	cerr, ok := err.(*Error)
	return ok && cerr.Kind == kind
}

// Error returns description of the error.
func (e *Error) Error() string {
	if e == nil {
		return "nil"
	}

	if e.ID == "" {
		e.ID = NewID()
	}

	b := new(bytes.Buffer)
	b.WriteString(e.Message)

	if e.Kind != 0 {
		pad(b, separator)
		b.WriteString(e.Kind.String())
	}

	if e.Err != nil {
		pad(b, separator)
		if cerr, ok := e.Err.(*Error); ok {
			b.WriteString(cerr.errorSkipID())
		} else {
			b.WriteString(e.Err.Error())
		}
	}
	b.WriteRune(' ')
	b.WriteRune('(')
	b.WriteString(e.ID)
	b.WriteRune(')')

	return b.String()
}

func (e *Error) errorSkipID() string {
	if e == nil {
		return "nil"
	}
	b := new(bytes.Buffer)
	b.WriteString(e.Message)

	if e.Kind != 0 {
		pad(b, separator)
		b.WriteString(e.Kind.String())
	}
	if e.Err != nil {
		pad(b, separator)
		if cerr, ok := e.Err.(*Error); ok {
			b.WriteString(cerr.errorSkipID())
		} else {
			b.WriteString(e.Err.Error())
		}
	}
	return b.String()
}

// StatusCode returns the HTTP status code corresponding to the error.
func (e *Error) StatusCode() int {
	switch e.Kind {
	case BadRequest:
		return http.StatusBadRequest
	case Unauthorized:
		return http.StatusUnauthorized
	case Forbidden:
		return http.StatusForbidden
	case Exist:
		return http.StatusConflict
	case NotFound:
		return http.StatusNotFound
	case Timeout:
		return http.StatusRequestTimeout
	case Internal:
		return http.StatusInternalServerError
	case ServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// MarshalJSON returns the JSON representation of an Error.
func (e *Error) MarshalJSON() (data []byte, err error) {
	var d = struct {
		ID      string `json:"id,omitempty"`
		Kind    Kind   `json:"kind"`
		Message string `json:"message,omitempty"`
	}{
		ID:      e.ID,
		Kind:    e.Kind,
		Message: e.Message,
	}
	return json.Marshal(d)
}

// UnmarshalJSON decodes a JSON encoded Error.
func (e *Error) UnmarshalJSON(data []byte) error {
	var d struct {
		ID      string `json:"id,omitempty"`
		Kind    Kind   `json:"kind"`
		Message string `json:"message,omitempty"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}

	*e = Error{
		ID:      d.ID,
		Kind:    d.Kind,
		Message: d.Message,
	}
	return nil
}

func JSON(w http.ResponseWriter, err error, statusCode ...int) {
	var e error
	var ok bool
	if e, ok = err.(*Error); !ok {
		e = New(err)
	}

	// check if the error can report its own status code
	code := http.StatusInternalServerError
	if sc, ok := e.(interface {
		StatusCode() int
	}); ok {
		code = sc.StatusCode()
	}

	// overwrite the status code if it's explicitly passed as argument
	if len(statusCode) > 0 {
		code = statusCode[0]
	}

	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(e)
}

// Temporary reports if an Error is temporary and
// whether the request can be retried.
func (e *Error) Temporary() bool {
	return e != nil && (e.Kind == Internal || e.Kind == Timeout)
}

// GetKind returns error kind determined
// by the specified HTTP status code.
func GetKind(statusCode int) Kind {
	switch statusCode {
	case http.StatusBadRequest:
		return BadRequest
	case http.StatusUnauthorized:
		return Unauthorized
	case http.StatusForbidden:
		return Forbidden
	case http.StatusConflict:
		return Exist
	case http.StatusNotFound:
		return NotFound
	case http.StatusRequestTimeout:
		return Timeout
	case http.StatusInternalServerError:
		return Internal
	case http.StatusServiceUnavailable:
		return ServiceUnavailable
	default:
		return Unknown
	}
}

// pad appends str to the buffer if the buffer already has some data.
func pad(b *bytes.Buffer, str string) {
	if b.Len() == 0 {
		return
	}
	b.WriteString(str)
}

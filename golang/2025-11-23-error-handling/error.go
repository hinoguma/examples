package exp

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"
)

func NewMyError(message string) *MyError {
	err := &MyError{
		err:        errors.New(message),
		stacktrace: NewStackTrace(2, 32),
	}
	return err
}

type MyError struct {
	// required
	errorType  ErrorType
	err        error
	stacktrace []StackTraceFrame

	// optional
	when      *time.Time
	requestId string
	tags      map[string]interface{}
	subErrors []*MyError
}

func (e *MyError) Error() string {
	return fmt.Sprintf("type:%s message:%s", e.errorType, e.err.Error())
}

func (e *MyError) SetWhen(t time.Time) *MyError {
	e.when = &t
	return e
}

func (e *MyError) SetRequestID(requestID string) *MyError {
	e.requestId = requestID
	return e
}

func (e *MyError) AddTag(key string, value any) *MyError {
	if e.tags == nil {
		e.tags = make(map[string]any)
	}
	e.tags[key] = value
	return e
}

func (e *MyError) AddSubError(errs ...error) *MyError {
	if len(errs) == 0 {
		return e
	}
	filtered := make([]*MyError, 0)
	for _, err := range errs {
		if err == nil {
			continue
		}
		filtered = append(filtered, ToMyError(err))
	}
	if len(filtered) == 0 {
		return e
	}
	if e.subErrors == nil {
		e.subErrors = make([]*MyError, 0)
	}
	e.subErrors = append(e.subErrors, filtered...)
	return e
}

type ErrorType string

func NewStackTrace(skip int, maxDepth int) []StackTraceFrame {
	if skip < 0 {
		skip = 0
	}
	skip += 2 // skip runtime.Callers and NewStackTrace
	if maxDepth <= 0 {
		return make([]StackTraceFrame, 0)
	}
	var trace []StackTraceFrame
	pc := make([]uintptr, maxDepth)
	cnt := runtime.Callers(skip, pc)
	frames := runtime.CallersFrames(pc[:cnt])
	for {
		frame, more := frames.Next()
		item := NewStackTraceFrame(frame)
		trace = append(trace, item)
		if !more {
			break
		}
	}
	return trace
}

type StackTraceFrame struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

func NewStackTraceFrame(f runtime.Frame) StackTraceFrame {
	return StackTraceFrame{
		File:     f.File,
		Line:     f.Line,
		Function: f.Function,
	}
}

func NewMyErrorByErr(e error) *MyError {
	err := &MyError{
		err:        e,
		stacktrace: NewStackTrace(2, 32),
	}
	return err
}

func ToMyError(err error) *MyError {
	if err == nil {
		return nil
	}
	me, ok := err.(*MyError)
	if ok {
		return me
	}
	return NewMyErrorByErr(err)
}

func With(err error, options ...WithFunc) error {
	if err == nil {
		return nil
	}
	for _, opt := range options {
		err = opt(err)
	}
	return err
}

type WithFunc func(err error) error

func RequestID(id string) WithFunc {
	return func(err error) error {
		me := ToMyError(err)
		me.requestId = id
		return me
	}
}

func When(t time.Time) WithFunc {
	return func(err error) error {
		me := ToMyError(err)
		me.when = &t
		return me
	}
}

func Tag(key string, value any) WithFunc {
	return func(err error) error {
		me := ToMyError(err)
		me.AddTag(key, value)
		return me
	}
}

func ToJsonString(err error) (string, error) {
	ej := ToErrorJson(err)
	b, err := json.Marshal(ej)
	if err != nil {
		return "{}", err
	}
	return string(b), nil
}

type ErrorJson struct {
	Type       string            `json:"type"`
	Message    string            `json:"message"`
	When       *time.Time        `json:"when,omitempty"`
	Request    string            `json:"request_id,omitempty"`
	Tags       interface{}       `json:"tags,omitempty"`
	StackTrace []StackTraceFrame `json:"stack_trace,omitempty"`
	SubErrs    []ErrorJson       `json:"sub_errors,omitempty"`
}

func ToErrorJson(err error) ErrorJson {
	if err == nil {
		return ErrorJson{}
	}
	me := ToMyError(err)
	ej := ErrorJson{
		Type:       string(me.errorType),
		Message:    me.err.Error(),
		When:       me.when,
		Request:    me.requestId,
		StackTrace: me.stacktrace,
	}
	if len(me.tags) > 0 {
		ej.Tags = me.tags
	}
	if len(me.subErrors) > 0 {
		subErrs := make([]ErrorJson, len(me.subErrors))
		for i, subErr := range me.subErrors {
			subErrs[i] = ToErrorJson(subErr)
		}
		ej.SubErrs = subErrs
	}
	return ej
}

// json出力零
/**
{
	  "type": "validation_error",
	  "message": "invalid input data",
	  "when": "2024-06-01T12:34:56Z",
	  "request_id": "abcd-1234-efgh-5678",
	  "tags": {
	    "field": "email",
	    "reason": "missing"
	  },
	  "stack_trace": [
	    {
	      "file": "/path/to/file.go",
	      "line": 42,
	      "function": "main.main"
	    },
	    {
	      "file": "/path/to/other_file.go",
	      "line": 27,
	      "function": "main.validateInput"
	    }
	  ],
	  "sub_errors": [
	    {
	      "type": "format_error",
	      "message": "email format is invalid"
	    },
	]
}


*/

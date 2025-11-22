package example

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"
)

var defaultMaxDepth = 32 // スタックトレースの深さ

func NewMyError(message string) *MyError {
	err := &MyError{
		err:        errors.New(message),
		stacktrace: NewStackTrace(2, defaultMaxDepth),
	}
	return err
}


type ErrorType string
const ErrorTypeNone ErrorType = ""

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


func NewMyErrorByErr(e error) *MyError {
	err := &MyError{
		err:        e,
		stacktrace: NewStackTrace(2, defaultMaxDepth),
	}
	return err
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








const ErrorTypeDataNotFound ErrorType = "DataNotFound"
const ErrorTypeConnectionFailed ErrorType = "ConnectionFailed"

func NewDataNotFoundError(id string) *MyError {
    err := NewMyError("data not found")
    err.AddTag("id", id)
    return err
}

func NewConnectionFailed(code int) *MyError {
    err := NewMyError("connection failed")
    err.SetType(ErrorTypeConnectionFailed)
    err.AddTag("code", code)
    return err
}

func IsDataNotFoundError(err error) bool {
    return IsType(err, ErrorTypeDataNotFound)
}

func IsConnectionFailed(err error) bool {
    return IsType(err, ErrorTypeConnectionFailed)
}


func IsType(err error, t ErrorType) bool {
	if err == nil {
		return false
	}
    // MyErrorでアサーションするのもOKですが
    // また違うカスタム構造体を作ったときのことを考えて
    // Type() ErrorType を実装していればなんでもOKとしています
	te, ok := err.(interface{ Type() ErrorType })
	if ok && te.Type() == t {
		return true
	}

    // errがラップされている場合Unwrapして内部のエラーもチェックします。
	switch x := err.(type) {
	case interface{ Unwrap() error }:
		return IsType(x.Unwrap(), t)
	case interface{ Unwrap() []error }:
		for _, subErr := range x.Unwrap() {
			if IsType(subErr, t) {
				return true
			}
		}
	}
	return false
}

func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
    me := ToMyError(err)
	if len(me.StackTrace()) == 0 {
		me.WithStackTrace()
	}
	me.SetErr(fmt.Errorf("%s: %w", msg, me.Unwrap()))
    return me
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



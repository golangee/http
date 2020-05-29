package http

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
)

// Error describes a (nested) server error
type Error struct {
	Id               string      `json:"id"`                         // Id is unique for a specific error, e.g. mydomain.not.assigned
	Message          string      `json:"message"`                    // Message is a string for the developer
	LocalizedMessage string      `json:"localizedMessage,omitempty"` // LocalizedMessage is something to display the user
	CausedBy         *Error      `json:"causedBy,omitempty"`         // CausedBy returns an optional root error
	Type             string      `json:"type,omitempty"`             // Type is a developer notice for the internal inspection
	Details          interface{} `json:"details,omitempty"`          // Details contains arbitrary payload
}

// ParseError tries to parse the response as json. In any case it returns an error.
func ParseError(reader io.Reader) *Error {
	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		return AsError(err)
	}

	res := &Error{}
	err = json.Unmarshal(buf, res)
	if err != nil {
		return AsError(err)
	}

	return res
}

// ID returns the unique error class id
func (c *Error) ID() string {
	return c.Id
}

// Error returns the message
func (c *Error) Error() string {
	return c.Message
}

// LocalizedError is like Error but translated or empty
func (c *Error) LocalizedError() string {
	return c.LocalizedMessage
}

// Class returns the technical type
func (c *Error) Class() string {
	return c.Type
}

// Payload returns the details
func (c *Error) Payload() interface{} {
	return c.Details
}

// Unwrap returns the cause or nil
func (c *Error) Unwrap() error {
	if c.CausedBy == nil { // otherwise error iface will not be nil, because of the type info in interface
		return nil
	}
	return c.CausedBy
}

func AsError(err error) *Error {
	if e, ok := err.(*Error); ok {
		return e
	}

	e := &Error{}
	e.Type = reflect.TypeOf(err).String()
	e.Message = err.Error()

	if code, ok := err.(interface{ ID() string }); ok {
		e.Id = code.ID()
	} else {
		e.Id = e.Type
	}

	if details, ok := err.(interface{ Payload() interface{} }); ok {
		e.Details = details
	}

	if localized, ok := err.(interface{ LocalizedError() string }); ok {
		e.LocalizedMessage = localized.LocalizedError()
	}

	if class, ok := err.(interface{ Class() string }); ok {
		e.Type = class.Class()
	}

	if wrapper, ok := err.(interface{ Unwrap() error }); ok {
		cause := wrapper.Unwrap()
		if cause != nil {
			tmp := AsError(cause)
			e.CausedBy = tmp
		}
	}

	return e
}

func marshalErrByte(err error) []byte {
	buf, err2 := json.Marshal(AsError(err))
	if err2 != nil {
		return []byte(fmt.Errorf("suppressed error by: %w", err2).Error())
	}
	return buf
}

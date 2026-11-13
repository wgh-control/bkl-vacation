package portal

import "fmt"

// Error is a stable API failure. Handlers copy Code to the response and the UI translates it.
type Error struct {
	Code    string
	Message string
	Status  int
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

func code(c, msg string, status int) *Error {
	return &Error{Code: c, Message: msg, Status: status}
}

func asError(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e
	}
	return nil
}

func wrap(err error, what string) error {
	if err == nil {
		return nil
	}
	if asError(err) != nil {
		return err
	}
	return fmt.Errorf("%s: %w", what, err)
}

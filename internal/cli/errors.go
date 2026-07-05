package cli

import "errors"

type quietExit interface {
	QuietExit() bool
}

type quietExitError struct {
	err error
}

func (e quietExitError) Error() string {
	return e.err.Error()
}

func (e quietExitError) Unwrap() error {
	return e.err
}

func (e quietExitError) QuietExit() bool {
	return true
}

func IsQuietExit(err error) bool {
	var quiet quietExit
	return errors.As(err, &quiet) && quiet.QuietExit()
}

var errNotEnabled = quietExitError{err: errors.New("not enabled")}

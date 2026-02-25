package apperrors

import (
	stderrors "errors"
	"fmt"
)

type Kind string

const (
	KindInvalidInput        Kind = "invalid_input"
	KindUnsupportedPlatform Kind = "unsupported_platform"
	KindPreflightFailure    Kind = "preflight_failure"
	KindGitFailure          Kind = "git_failure"
	KindFilesystemFailure   Kind = "filesystem_failure"
	KindRollbackFailure     Kind = "rollback_failure"
)

const (
	ExitOK                  = 0
	ExitUnknown             = 1
	ExitInvalidInput        = 2
	ExitUnsupportedPlatform = 3
	ExitPreflightFailure    = 4
	ExitGitFailure          = 5
	ExitFilesystemFailure   = 6
	ExitRollbackFailure     = 7
)

type Error struct {
	Kind    Kind
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func New(kind Kind, message string) *Error {
	return &Error{Kind: kind, Message: message}
}

func Wrap(kind Kind, message string, cause error) *Error {
	return &Error{Kind: kind, Message: message, Cause: cause}
}

func IsKind(err error, kind Kind) bool {
	var appErr *Error
	if !stderrors.As(err, &appErr) {
		return false
	}
	return appErr.Kind == kind
}

func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var appErr *Error
	if !stderrors.As(err, &appErr) {
		return ExitUnknown
	}

	switch appErr.Kind {
	case KindInvalidInput:
		return ExitInvalidInput
	case KindUnsupportedPlatform:
		return ExitUnsupportedPlatform
	case KindPreflightFailure:
		return ExitPreflightFailure
	case KindGitFailure:
		return ExitGitFailure
	case KindFilesystemFailure:
		return ExitFilesystemFailure
	case KindRollbackFailure:
		return ExitRollbackFailure
	default:
		return ExitUnknown
	}
}

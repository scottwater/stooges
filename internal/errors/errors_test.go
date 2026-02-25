package apperrors

import (
	"errors"
	"testing"
)

func TestExitCodeMapping(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{err: nil, code: ExitOK},
		{err: New(KindInvalidInput, "bad"), code: ExitInvalidInput},
		{err: New(KindUnsupportedPlatform, "bad"), code: ExitUnsupportedPlatform},
		{err: New(KindPreflightFailure, "bad"), code: ExitPreflightFailure},
		{err: New(KindGitFailure, "bad"), code: ExitGitFailure},
		{err: New(KindFilesystemFailure, "bad"), code: ExitFilesystemFailure},
		{err: New(KindRollbackFailure, "bad"), code: ExitRollbackFailure},
		{err: errors.New("plain"), code: ExitUnknown},
	}

	for _, tc := range cases {
		if got := ExitCode(tc.err); got != tc.code {
			t.Fatalf("expected %d got %d for err %v", tc.code, got, tc.err)
		}
	}
}

func TestIsKind(t *testing.T) {
	err := Wrap(KindGitFailure, "git failed", errors.New("boom"))
	if !IsKind(err, KindGitFailure) {
		t.Fatal("expected git failure kind")
	}
	if IsKind(err, KindInvalidInput) {
		t.Fatal("did not expect invalid input kind")
	}
}

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newShellInitCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "shell-init [bash|zsh|sh]",
		Short: "Print shell integration for auto-cd after add/branch/fork/track",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("accepts at most 1 arg(s), received %d", len(args))
			}
			if len(args) == 0 {
				return nil
			}
			switch strings.TrimSpace(args[0]) {
			case "", "bash", "zsh", "sh":
				return nil
			default:
				return fmt.Errorf("unsupported shell %q (expected bash, zsh, or sh)", args[0])
			}
		},
		Run: func(_ *cobra.Command, _ []string) {
			_, _ = streams.Out.Write([]byte(stoogesShellInitScript))
		},
	}
}

const stoogesShellInitScript = `stooges() {
  _stooges_cd_file="$(mktemp -t stooges-cd.XXXXXX 2>/dev/null || mktemp)" || return 1
  STOOGES_CD_FILE="${_stooges_cd_file}" command stooges "$@"
  _stooges_status=$?
  if [ "${_stooges_status}" -eq 0 ] && [ -s "${_stooges_cd_file}" ]; then
    if ! _stooges_cd_target="$(cat "${_stooges_cd_file}")"; then
      printf 'stooges: warning: failed to read auto-cd target from %s\n' "${_stooges_cd_file}" >&2
    elif [ -n "${_stooges_cd_target}" ]; then
      if ! cd "${_stooges_cd_target}"; then
        _stooges_status=$?
        printf 'stooges: warning: failed to cd to %s\n' "${_stooges_cd_target}" >&2
      fi
    fi
  fi
  if ! rm -f "${_stooges_cd_file}"; then
    printf 'stooges: warning: failed to remove temp file %s\n' "${_stooges_cd_file}" >&2
  fi
  return "${_stooges_status}"
}
`

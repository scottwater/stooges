package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func Confirm(reader *bufio.Reader, out io.Writer, preview string) (bool, error) {
	if preview != "" {
		fmt.Fprintln(out, preview)
	}
	fmt.Fprint(out, "Proceed? [y/N]: ")
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func ConfirmIO(in io.Reader, out io.Writer, preview string) (bool, error) {
	return Confirm(bufio.NewReader(in), out, preview)
}

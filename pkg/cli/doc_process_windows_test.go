package cli_test

import (
	"os/exec"
	"strconv"
	"time"
)

// Documentation examples currently skip Windows, but keep cancellation bounded
// when this helper is used by Windows tests in the future.
func configureExampleProcess(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	}
	cmd.WaitDelay = 5 * time.Second
}

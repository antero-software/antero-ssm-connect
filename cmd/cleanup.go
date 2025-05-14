package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/antero-software/antero-ssm-connect/internal/tunnel"
)

func CleanupAndExit() {
	if tunnel.CurrentPid != 0 {
		fmt.Println("\n🔴 Closing port-forward session...")
		_ = syscall.Kill(-tunnel.CurrentPid, syscall.SIGKILL)
	}
	os.Exit(0)
}

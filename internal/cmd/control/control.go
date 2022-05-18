package control

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/canonical/microcluster/internal/sys"
)

// CmdControl has functions that are common to the microcluster-admin, microcluster-cellctl, and microcluster-regionctl
// command line tools.
type CmdControl struct {
	cmd *cobra.Command //nolint:structcheck,unused // FIXME: Remove the nolint flag when this is in use.

	FlagHelp       bool
	FlagVersion    bool
	FlagLogDebug   bool
	FlagLogVerbose bool
	FlagStateDir   string
}

// GetStateDir determines the state directory of the daemon via environment variable or cli flag.
func (c *CmdControl) GetStateDir() (*sys.OS, error) {
	if c.FlagStateDir != "" {
		return sys.DefaultOS(c.FlagStateDir, false)
	}

	dir := sys.StateDir
	envDir := os.Getenv(dir)
	if envDir != "" {
		return sys.DefaultOS(envDir, false)
	}

	return nil, fmt.Errorf("Invalid state directory")
}

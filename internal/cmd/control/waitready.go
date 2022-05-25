package control

import (
	"context"
	"fmt"
	"time"

	"github.com/lxc/lxd/shared/api"
	"github.com/spf13/cobra"

	"github.com/canonical/microcluster/internal/logger"
	"github.com/canonical/microcluster/internal/rest/client"
)

// RunWaitready blocks execution until the cell/region daemon is ready to accept requests.
func (c *CmdControl) RunWaitready(flagTimeout int, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cmd.Help()
	}

	finger := make(chan error, 1)
	var errLast error
	go func() {
		for i := 0; ; i++ {
			// Start logging only after the 10'th attempt (about 5
			// seconds). Then after the 30'th attempt (about 15
			// seconds), log only only one attempt every 10
			// attempts (about 5 seconds), to avoid being too
			// verbose.
			doLog := false
			if i > 10 {
				doLog = i < 30 || ((i % 10) == 0)
			}

			if doLog {
				logger.Debugf("Connecting to LXD Cloud daemon (attempt %d)", i)
			}

			os, err := c.GetStateDir()
			if err != nil {
				errLast = err
				if doLog {
					logger.Debugf("Failed to get state dir (attempt %d): %v", i, err)
				}

				time.Sleep(500 * time.Millisecond)
				continue
			}

			d, err := client.New(os.ControlSocket(), nil, nil, false)
			if err != nil {
				errLast = err
				if doLog {
					logger.Debugf("Failed connecting to LXD Cloud daemon (attempt %d): %v", i, err)
				}

				time.Sleep(500 * time.Millisecond)
				continue
			}

			if doLog {
				logger.Debugf("Checking if LXD Cloud daemon is ready (attempt %d)", i)
			}

			err = d.QueryStruct(context.Background(), "GET", client.ControlEndpoint, api.NewURL().Path("ready"), nil, nil)
			if err != nil {
				errLast = err
				if doLog {
					logger.Debugf("Failed to check if LXD Cloud daemon is ready (attempt %d): %v", i, err)
				}

				time.Sleep(500 * time.Millisecond)
				continue
			}

			finger <- nil
			return
		}
	}()

	if flagTimeout > 0 {
		select {
		case <-finger:
			break
		case <-time.After(time.Second * time.Duration(flagTimeout)):
			return fmt.Errorf("LXD Cloud still not running after %ds timeout (%v)", flagTimeout, errLast)
		}
	} else {
		<-finger
	}

	return nil
}

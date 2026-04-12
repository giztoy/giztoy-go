package client

import (
	"fmt"

	"github.com/giztoy/giztoy-go/internal/clicontext"
	"github.com/giztoy/giztoy-go/pkg/gizclaw"
)

func DialFromContext(name string) (*gizclaw.Client, error) {
	store, err := clicontext.DefaultStore()
	if err != nil {
		return nil, err
	}
	var cliCtx *clicontext.CLIContext
	if name != "" {
		cliCtx, err = store.LoadByName(name)
	} else {
		cliCtx, err = store.Current()
	}
	if err != nil {
		return nil, err
	}
	if cliCtx == nil {
		return nil, fmt.Errorf("no active context; run 'giztoy context create' first")
	}
	serverPK, err := cliCtx.ServerPublicKey()
	if err != nil {
		return nil, fmt.Errorf("invalid server public key: %w", err)
	}
	return gizclaw.Dial(cliCtx.KeyPair, cliCtx.Config.Server.Address, serverPK)
}

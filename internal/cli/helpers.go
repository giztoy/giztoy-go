package cli

import (
	"fmt"

	"github.com/giztoy/giztoy-go/internal/client"
	gctx "github.com/giztoy/giztoy-go/internal/context"
)

func dialFromContext(name string) (*client.Client, error) {
	store, err := gctx.DefaultStore()
	if err != nil {
		return nil, err
	}
	var ctx *gctx.Context
	if name != "" {
		ctx, err = store.LoadByName(name)
	} else {
		ctx, err = store.Current()
	}
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, fmt.Errorf("no active context; run 'giztoy context create' first")
	}
	serverPK, err := ctx.ServerPublicKey()
	if err != nil {
		return nil, fmt.Errorf("invalid server public key: %w", err)
	}
	return client.Dial(ctx.KeyPair, ctx.Config.Server.Address, serverPK)
}

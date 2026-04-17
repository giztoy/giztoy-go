package servicerun

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/GizClaw/gizclaw-go/cmd/internal/server"
	"github.com/GizClaw/gizclaw-go/cmd/internal/service"
	kservice "github.com/kardianos/service"
)

type program struct {
	workspace string

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan error
}

var serviceStopTimeout = 5 * time.Second

func RunWorkspaceService(workspace string) error {
	prg := &program{workspace: workspace}
	svc, err := service.NewService(workspace, prg)
	if err != nil {
		return err
	}
	return svc.Run()
}

func (p *program) Start(s kservice.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	p.mu.Lock()
	p.cancel = cancel
	p.done = done
	p.mu.Unlock()

	go func() {
		err := server.ServeContext(ctx, p.workspace, server.ServeOptions{
			Force:          true,
			ServiceManaged: true,
		})
		done <- err
		if err != nil {
			_ = s.Stop()
		}
	}()

	return nil
}

func (p *program) Stop(kservice.Service) error {
	p.mu.Lock()
	cancel := p.cancel
	done := p.done
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done == nil {
		return nil
	}

	select {
	case err := <-done:
		if err == nil || errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("service: server stopped with error: %w", err)
	case <-time.After(serviceStopTimeout):
		return errors.New("service: timeout waiting for server shutdown")
	}
}

package daemon

import (
	"context"
	"log"
	"sync"

	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/lifecycle"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/so"
)

type Daemon struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (d *Daemon) Stop() {
	d.cancel()
	d.wg.Wait()
	logging.L().Debug().Msg("daemon stopped")
}

func startLifecycle(parent context.Context) (func(), error) {
	logging.L().Debug().Msg("Starting lifecycle tracer...")

	ctx, cancel := context.WithCancel(parent)

	cfg := lifecycle.Config{} // if you have any config, set it here
	loader, err := lifecycle.LoadProcExitObjects(cfg)
	if err != nil {
		cancel()
		return nil, err
	}
	if err := loader.Attach(cfg); err != nil {
		cancel()
		log.Fatal(err)
	}

	reader, err := lifecycle.NewRingbufReader(loader)
	if err != nil {
		cancel()
		loader.Close()
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		lifecycle.RunProcExitRd(ctx, reader, cfg)
	}()

	// cleanup function
	return func() {
		cancel()
		reader.Close()
		<-done
		loader.Close()
	}, nil
}

func startCuInitTracer(parent context.Context) (func(), error) {
	logging.L().Debug().Msg("Starting cuInit tracer...")
	ctx, cancel := context.WithCancel(parent)

	cfg := lifecycle.Config{}
	loader, err := lifecycle.LoadCuInitObjects(cfg)
	if err != nil {
		cancel()
		return nil, err
	}

	links, err := loader.Attach(internal.CudaSo, loader)
	if err != nil {
		cancel()
		loader.Close()
		return nil, err
	}

	reader, err := lifecycle.NewCuInitRingbufReader(loader)
	if err != nil {
		cancel()
		for _, l := range links {
			l.Close()
		}
		loader.Close()
		return nil, err
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		lifecycle.RunCuInitRd(ctx, reader, cfg)
	}()

	// cleanup
	return func() {
		cancel()
		reader.Close()
		for _, link := range links {
			link.Close()
		}
		<-done
		loader.Close()
	}, nil
}

func Start(parent context.Context) (*Daemon, error) {
	parent, cancel := context.WithCancel(parent)
	d := &Daemon{
		ctx:    parent,
		cancel: cancel,
	}
	if err := so.InitFromSharedObject(internal.CudaSo); err != nil {
		return nil, err
	}
	// lifecycle tracer
	d.wg.Go(func() {
		startLifecycle(parent)
	})

	d.wg.Go(func() {
		startCuInitTracer(parent)
	})
	logging.L().Info().Msg("daemon started")
	return d, nil
}

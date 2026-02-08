package daemon

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/lifecycle"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
)

type Daemon struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewDaemon(ctx context.Context) *Daemon {
	return &Daemon{
		ctx: ctx,
	}
}

func (d *Daemon) Go(fn func(ctx context.Context)) {
	d.wg.Go(func() {
		fn(d.ctx)
	})
}

func (d *Daemon) Wait() {
	<-d.ctx.Done()
	logging.L().Debug().Msg("daemon shutting down ...")
	d.wg.Wait()
	logging.L().Debug().Msg("all daemon goroutines exited")

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
		go func() {
			<-ctx.Done()
			reader.Close()
		}()
		lifecycle.RunCuInitRd(ctx, reader, cfg)
	}()

	// cleanup
	return func() {
		cancel()
		for _, link := range links {
			link.Close()
		}
		<-done
		loader.Close()
	}, nil
}

func Start(parent context.Context) (error, *Daemon) {
	daemon := NewDaemon(parent)
	daemon.Go(func(ctx context.Context) {
		_, err := startCuInitTracer(ctx)
		if err != nil {
			os.Exit(1)
		}
	})
	daemon.Go(func(ctx context.Context) {
		_, err := startLifecycle(ctx)
		if err != nil {
			os.Exit(1)
		}
	})
	return nil, daemon
}

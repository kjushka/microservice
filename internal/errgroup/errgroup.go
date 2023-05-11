package errgroup

import (
	"context"
	"errors"
	logger2 "github.com/kjushka/microservice-gen/internal/logger"
	"runtime/debug"

	"go.uber.org/zap"
	eg "golang.org/x/sync/errgroup"
)

type Group struct {
	g   *eg.Group
	log *zap.SugaredLogger
}

func WithContext(ctx context.Context) (*Group, context.Context) {
	g, retCtx := eg.WithContext(ctx)
	log := logger2.FromContext(ctx)
	return &Group{g: g, log: log}, retCtx
}

func (g *Group) Go(f func() error) {
	g.g.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				g.logger().With("stack", string(debug.Stack())).Error("recovered from panic")
				err = errors.New("recovered from panic")
			}
		}()
		err = f()
		return
	})
}

func (g *Group) logger() *zap.SugaredLogger {
	if g.log != nil {
		return g.log
	}
	return logger2.Logger()
}

func (g *Group) Wait() error {
	return g.g.Wait()
}

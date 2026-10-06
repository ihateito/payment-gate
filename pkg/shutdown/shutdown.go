package shutdown

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// CleanupFunc is a function executed during shutdown.
type CleanupFunc func(ctx context.Context) error

// Coordinator manages graceful application termination.
type Coordinator struct {
	timeout time.Duration
	logger  *slog.Logger
	mu      sync.Mutex
	tasks   []namedTask
}

type namedTask struct {
	name string
	fn   CleanupFunc
}

// NewCoordinator creates a new shutdown coordinator.
func NewCoordinator(timeout time.Duration, l *slog.Logger) *Coordinator {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if l == nil {
		l = slog.Default()
	}
	return &Coordinator{
		timeout: timeout,
		logger:  l.With("component", "graceful_shutdown"),
	}
}

// Register registers a cleanup task executed upon shutdown. Tasks run in reverse order (LIFO).
func (c *Coordinator) Register(name string, fn CleanupFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tasks = append(c.tasks, namedTask{name: name, fn: fn})
}

// ListenAndWait blocks until a termination signal (SIGINT, SIGTERM) is intercepted,
// then executes all registered cleanup tasks within the configured timeout.
func (c *Coordinator) ListenAndWait(parentCtx context.Context) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigChan:
		c.logger.Info("shutdown signal received", "signal", sig.String())
	case <-parentCtx.Done():
		c.logger.Info("context cancelled, initiating shutdown")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	c.mu.Lock()
	tasks := make([]namedTask, len(c.tasks))
	copy(tasks, c.tasks)
	c.mu.Unlock()

	// Execute in reverse registration order (LIFO)
	for i := len(tasks) - 1; i >= 0; i-- {
		task := tasks[i]
		c.logger.Info("executing cleanup task", "task", task.name)
		start := time.Now()
		if err := task.fn(shutdownCtx); err != nil {
			c.logger.Error("cleanup task failed", "task", task.name, "duration", time.Since(start), "error", err)
		} else {
			c.logger.Info("cleanup task completed", "task", task.name, "duration", time.Since(start))
		}
	}

	c.logger.Info("graceful shutdown finished")
	return nil
}

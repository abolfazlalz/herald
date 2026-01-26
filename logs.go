package herald

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

const (
	reset  = "\033[0m"
	red    = "\033[31m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	gray   = "\033[90m"
	cyan   = "\033[36m"
	green  = "\033[32m"
)

func levelColor(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return red
	case l >= slog.LevelWarn:
		return yellow
	case l >= slog.LevelInfo:
		return blue
	default:
		return gray
	}
}

type colorHandler struct {
	h slog.Handler
}

func (c *colorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return c.h.Enabled(ctx, level)
}

func (h *colorHandler) Handle(_ context.Context, r slog.Record) error {
	color := levelColor(r.Level)

	msg := fmt.Sprintf(
		"%s%s %s%-5s%s %s",
		gray,
		r.Time.Format("15:04:05"),
		color,
		r.Level,
		reset,
		r.Message,
	)

	r.Attrs(func(a slog.Attr) bool {
		// attributes
		a.Value = a.Value.Resolve()

		msg += fmt.Sprintf(
			" %s%s%s=%s\"%v\"%s",
			cyan, a.Key, reset,
			green, a.Value.String(), reset,
		)
		return true
	})

	fmt.Println(msg)
	return nil
}

func (h *colorHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *colorHandler) WithGroup(string) slog.Handler      { return h }

type Log interface {
	Debug(ctx context.Context, msg string, args ...any)
	Info(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, args ...any)
	Warn(ctx context.Context, msg string, args ...any)
}

type implLogs struct {
	logs *slog.Logger
}

func NewLogs(logger *slog.Logger) Log {
	return &implLogs{logs: logger}
}

func NewDiscardLogs() Log {
	return NewLogs(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func NewLogger(option *slog.HandlerOptions) Log {
	base := slog.NewTextHandler(os.Stdout, option)
	logger := NewLogs(slog.New(&colorHandler{h: base}))
	return logger
}

func (impl implLogs) Debug(ctx context.Context, msg string, args ...any) {
	impl.logs.DebugContext(ctx, msg, args...)
}

func (impl implLogs) Info(ctx context.Context, msg string, args ...any) {
	impl.logs.InfoContext(ctx, msg, args...)
}

func (impl implLogs) Error(ctx context.Context, msg string, args ...any) {
	impl.logs.ErrorContext(ctx, msg, args...)
}

func (impl implLogs) Warn(ctx context.Context, msg string, args ...any) {
	impl.logs.WarnContext(ctx, msg, args...)
}

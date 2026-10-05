package log

import (
	"log/slog"
	"os"
)

func Default() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
}

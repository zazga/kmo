package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type Logs struct {
	Info  *slog.Logger
	Error *slog.Logger
	files []io.Closer
}

func Open() (*Logs, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	dir := filepath.Join(home, ".local", "state", "kmo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	infoFile, err := os.OpenFile(filepath.Join(dir, "kmo.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	errFile, err := os.OpenFile(filepath.Join(dir, "kmo.error.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		infoFile.Close()
		return nil, err
	}
	return &Logs{
		Info:  slog.New(slog.NewJSONHandler(infoFile, &slog.HandlerOptions{Level: slog.LevelInfo})),
		Error: slog.New(slog.NewJSONHandler(errFile, &slog.HandlerOptions{Level: slog.LevelWarn})),
		files: []io.Closer{infoFile, errFile},
	}, nil
}

func (l *Logs) Close() {
	for _, f := range l.files {
		_ = f.Close()
	}
}

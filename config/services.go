package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type Logger struct {
	file *os.File
	*slog.Logger
}

func (receiver *Logger) Close() error {
	if receiver.file != nil {
		return receiver.file.Close()
	}

	return nil
}

func openLogFile() (*os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed getting home dir: %w", err)
	}

	filePath := filepath.Join(home, ".local", "share", guiVendor, guiName, "log.txt")
	if _, err := os.Stat(filepath.Dir(filePath)); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed creating log dir %s: %w", filepath.Dir(filePath), err)
		}
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed opening file %s: %w", filePath, err)
	}

	return file, nil
}

func NewLoggerWithoutFile() *Logger {
	return &Logger{
		Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
}

func NewLogger() *Logger {
	file, err := openLogFile()
	var writer io.Writer

	if err == nil && file != nil {
		writer = io.MultiWriter(os.Stderr, file)
	} else {
		writer = os.Stderr
	}

	logger := slog.New(slog.NewTextHandler(writer, nil))
	if err != nil {
		logger.Error("could not open log file, logging only to stderr", slog.String("error", err.Error()))
	}

	return &Logger{file: file, Logger: logger}
}

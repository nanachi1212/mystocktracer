package runtimelog

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const DefaultMaxBytes int64 = 5 * 1024 * 1024
const DefaultBackups = 5

var masks = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile("(?i)\\bBearer\\s+[^\\s\"',;}]+"), "Bearer <redacted>"},
	{regexp.MustCompile("(?i)([?&](?:key|token|api[_-]?key|authorization|cookie|credential|password|secret)=)[^&\\s\"']*"), "1<redacted>"},
	{regexp.MustCompile("(?i)([\"']?\\b(?:[\\w-]*[_-])?(?:api[_-]?key|token|authorization|cookie|credential|password|secret)[\"']?\\s*[:=]\\s*)(?:\"[^\"\\r\\n]*\"|'[^'\\r\\n]*'|[^\\s,;&}]+)"), "1<redacted>"},
}

func Redact(message string) string {
	for _, mask := range masks {
		message = mask.pattern.ReplaceAllString(message, mask.replacement)
	}
	characters := []rune(message)
	if len(characters) > 16384 {
		return string(characters[:16384])
	}
	return message
}

// Each Write is one independently persisted, redacted record. Opening within the
// mutex avoids retaining a stale file descriptor across rotation or backup.
type Writer struct {
	mu       sync.Mutex
	filename string
	limit    int64
	retain   int
	closed   bool
}

func NewWriter(directory, fileName string, maxBytes int64, backups int) (*Writer, error) {
	if strings.TrimSpace(directory) == "" || fileName == "" || fileName == "." || filepath.Base(fileName) != fileName {
		return nil, errors.New("invalid runtime log path")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	if backups < 0 {
		backups = DefaultBackups
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	w := &Writer{filename: filepath.Join(directory, fileName), limit: maxBytes, retain: backups}
	f, err := os.OpenFile(w.filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	return w, nil
}
func (w *Writer) Path() string {
	if w == nil {
		return ""
	}
	return w.filename
}
func (w *Writer) Close() error {
	if w != nil {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.closed = true
	}
	return nil
}
func (w *Writer) Write(input []byte) (int, error) {
	if w == nil {
		return 0, io.ErrClosedPipe
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, io.ErrClosedPipe
	}
	record := []byte(Redact(string(input)))
	if int64(len(record)) > w.limit {
		record = record[:w.limit]
	}
	stat, err := os.Stat(w.filename)
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	if stat != nil && stat.Size()+int64(len(record)) > w.limit {
		for index := w.retain; index >= 0; index-- {
			current := w.filename
			if index > 0 {
				current = fmt.Sprintf("%s.%d", w.filename, index)
			}
			if index == w.retain {
				if err := os.Remove(current); err != nil && !os.IsNotExist(err) {
					return 0, err
				}
			} else if err := os.Rename(current, fmt.Sprintf("%s.%d", w.filename, index+1)); err != nil && !os.IsNotExist(err) {
				return 0, err
			}
		}
	}
	file, err := os.OpenFile(w.filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return 0, err
	}
	_, writeErr := file.Write(record)
	closeErr := file.Close()
	if writeErr != nil {
		return 0, writeErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	return len(input), nil
}

type sanitizedMirror struct {
	file    *Writer
	console io.Writer
}

func (m sanitizedMirror) Write(input []byte) (int, error) {
	clean := []byte(Redact(string(input)))
	// Neither stderr nor the file receives the unredacted caller buffer.
	_, consoleErr := m.console.Write(clean)
	_, fileErr := m.file.Write(clean)
	if fileErr != nil {
		return 0, fileErr
	}
	if consoleErr != nil {
		return 0, consoleErr
	}
	return len(input), nil
}
func ConfigureStandard(directory, component string) (*log.Logger, io.Closer, error) {
	writer, err := NewWriter(directory, component+".log", DefaultMaxBytes, DefaultBackups)
	if err != nil {
		return nil, nil, err
	}
	output := sanitizedMirror{writer, os.Stderr}
	flags := log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC
	log.SetOutput(output)
	log.SetFlags(flags)
	return log.New(output, "", flags), writer, nil
}

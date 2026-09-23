package runtimelog

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggingStorageContract(t *testing.T) {
	for _, retained := range []int{0, 2} {
		t.Run(fmt.Sprintf("retain-%d", retained), func(t *testing.T) {
			root := t.TempDir()
			writer, err := NewWriter(root, "events.log", 128, retained)
			if err != nil {
				t.Fatal(err)
			}
			for n := 0; n < 15; n++ {
				record := []byte(fmt.Sprintf("record-%02d %s\n", n, strings.Repeat("x", 160)))
				written, err := writer.Write(record)
				if err != nil || written != len(record) {
					t.Fatalf("write=%d err=%v", written, err)
				}
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != retained+1 {
				t.Fatalf("files=%d want=%d", len(entries), retained+1)
			}
			for index := 0; index <= retained; index++ {
				name := "events.log"
				if index > 0 {
					name += fmt.Sprintf(".%d", index)
				}
				contents, err := os.ReadFile(filepath.Join(root, name))
				if err != nil {
					t.Fatal(err)
				}
				if len(contents) > 128 || !bytes.Contains(contents, []byte(fmt.Sprintf("record-%02d", 14-index))) {
					t.Fatalf("invalid retained record %s", name)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write([]byte("closed")); err != io.ErrClosedPipe {
				t.Fatalf("closed write=%v", err)
			}
		})
	}
}

func TestSecretRedactionContract(t *testing.T) {
	vectors := [][2]string{
		{"Bearer synthetic-a", "Bearer <redacted>"},
		{"token=synthetic-b", "token=<redacted>"},
		{"api_key:synthetic-c", "api_key:<redacted>"},
		{"cookie=synthetic-d", "cookie=<redacted>"},
		{"GET /api?token=synthetic-e&mode=test", "GET /api?token=<redacted>&mode=test"},
		{"{\"credential\":\"synthetic-f\"}", "{\"credential\":<redacted>}"},
	}
	root := t.TempDir()
	writer, err := NewWriter(root, "redacted.log", DefaultMaxBytes, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, vector := range vectors {
		if got := Redact(vector[0]); got != vector[1] {
			t.Errorf("redaction=%q want=%q", got, vector[1])
		}
		if _, err := writer.Write([]byte(vector[0] + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	persisted, err := os.ReadFile(filepath.Join(root, "redacted.log"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(persisted, []byte("synthetic-")) {
		t.Fatal("synthetic secret reached storage")
	}
	if bytes.Count(persisted, []byte("<redacted>")) != len(vectors) {
		t.Fatal("missing redacted record")
	}
	if !bytes.Contains(persisted, []byte("mode=test")) {
		t.Fatal("diagnostic context lost")
	}
}

func TestLogDestinationRejectsTraversal(t *testing.T) {
	for _, name := range []string{"", ".", "../escape.log"} {
		if _, err := NewWriter(t.TempDir(), name, 128, 1); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
}

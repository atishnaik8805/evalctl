package dataset

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return p
}

func TestLoadJSONL(t *testing.T) {
	p := writeTemp(t, "tasks.jsonl", `{"id":"a","input":"hi","metadata":{"expected":"HI"}}
{"id":"b","input":"yo"}
`)
	tasks, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	if tasks[0].ID != "a" {
		t.Errorf("tasks[0].ID = %q, want a", tasks[0].ID)
	}
	if string(tasks[1].Metadata) != "{}" {
		t.Errorf("tasks[1].Metadata = %s, want {}", tasks[1].Metadata)
	}
}

func TestLoadJSON(t *testing.T) {
	p := writeTemp(t, "tasks.json", `[{"id":"a","input":"hi"},{"id":"b","input":"yo"}]`)
	tasks, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
}

func TestLoadCSV(t *testing.T) {
	p := writeTemp(t, "tasks.csv", "id,input\na,hi\nb,yo\n")
	tasks, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	if string(tasks[0].Input) != `"hi"` {
		t.Errorf("tasks[0].Input = %s, want \"hi\"", tasks[0].Input)
	}
}

func TestLoadDuplicateIDRejected(t *testing.T) {
	p := writeTemp(t, "tasks.jsonl", `{"id":"a","input":"hi"}
{"id":"a","input":"yo"}
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for duplicate id, got nil")
	}
}

func TestLoadMalformedRowRejected(t *testing.T) {
	p := writeTemp(t, "tasks.jsonl", `{"id":"a","input":"hi"}
{"input":"missing id"}
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for malformed row, got nil")
	}
}

func TestLoadUnsupportedExtension(t *testing.T) {
	p := writeTemp(t, "tasks.txt", "id,input\na,hi\n")
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for unsupported extension, got nil")
	}
}

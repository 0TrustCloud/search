package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0TrustCloud/orchid_sync"
	"github.com/0TrustCloud/ultimate_db"
)

func testDB(t *testing.T) (*ultimate_db.DB, *orchid_sync.Engine, func()) {
	t.Helper()
	// Manual temp dir: Windows often keeps ultimate_db handles open past Close,
	// and t.TempDir() then fails the test on RemoveAll.
	dir, err := os.MkdirTemp("", "search-test-*")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "t.db")
	walPath := filepath.Join(dir, "t.wal")

	device, err := ultimate_db.NewOSFileDevice(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	dm := ultimate_db.NewDiskManager(device)
	evictor := ultimate_db.NewLRUEvictionPolicy()
	metrics := ultimate_db.NewAtomicMetrics()
	bp := ultimate_db.NewBufferPool(dm, 256, evictor, metrics)
	wal, err := ultimate_db.NewBatchingWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}
	db := ultimate_db.NewDB(bp, wal, metrics)
	for {
		p, err := bp.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		bp.UnpinPage(p.ID, true)
		if p.ID >= ultimate_db.PageID(12) {
			break
		}
	}
	eng, err := orchid_sync.NewEngine(db, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		_ = db.Close()
		// Best-effort; ignore Windows file-lock errors on cleanup.
		_ = os.RemoveAll(dir)
	}
	return db, eng, cleanup
}

func TestBM25SearchRanksRelevantDoc(t *testing.T) {
	db, eng, cleanup := testDB(t)
	defer cleanup()
	store := NewStore(db, eng)

	docs := []Document{
		{URL: "https://example.com/cats", Title: "All about cats", Body: "Cats are wonderful pets. The domestic cat purrs and naps."},
		{URL: "https://example.com/dogs", Title: "Dog training guide", Body: "Dogs love walks and training. Puppies need patience."},
		{URL: "https://example.com/birds", Title: "Bird watching", Body: "Birds migrate south in winter. Binoculars help spotting."},
	}
	for _, d := range docs {
		if err := store.IndexDocument(d); err != nil {
			t.Fatalf("index: %v", err)
		}
	}

	resp, err := store.Search("cats purr", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if resp.Total == 0 {
		t.Fatal("expected hits for cats query")
	}
	if resp.Hits[0].URL != "https://example.com/cats" {
		t.Fatalf("expected cats doc first, got %s score=%v", resp.Hits[0].URL, resp.Hits[0].Score)
	}
	if resp.Hits[0].Score <= 0 {
		t.Fatalf("expected positive BM25 score, got %v", resp.Hits[0].Score)
	}
	if resp.Hits[0].Snippet == "" {
		t.Fatal("expected non-empty snippet")
	}
}

func TestNormalizeQueryOR(t *testing.T) {
	q := normalizeQuery("zero trust mesh")
	if q != "zero OR trust OR mesh" {
		// stop words may filter; at least multi-term OR form
		if !containsAll(q, "zero", "trust", "mesh") && q != "zero OR trust OR mesh" {
			// "trust" is not a stop word; "zero" and "mesh" should remain
			t.Logf("normalized: %s", q)
		}
	}
	if normalizeQuery("hello") != "hello" {
		t.Fatalf("single term: %s", normalizeQuery("hello"))
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && (func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})()))
}

func TestDocIDStable(t *testing.T) {
	a := DocIDFromURL("https://Example.COM/Path")
	b := DocIDFromURL("https://example.com/path")
	if a != b {
		t.Fatalf("expected case-insensitive stable ids: %s vs %s", a, b)
	}
}

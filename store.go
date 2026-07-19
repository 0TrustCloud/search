package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/0TrustCloud/orchid_sync"
	"github.com/0TrustCloud/ultimate_db"
)

const docPrefix = "doc:"

// Document is a web page (or other text resource) in the search corpus.
type Document struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Body        string    `json:"body"`
	Domain      string    `json:"domain"`
	IndexedAt   time.Time `json:"indexed_at"`
	Source      string    `json:"source,omitempty"` // seed | submit | api | crawl
}

// Hit is a ranked search result with a display snippet.
type Hit struct {
	ID          string  `json:"id"`
	URL         string  `json:"url"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Domain      string  `json:"domain"`
	Snippet     string  `json:"snippet"`
	Score       float64 `json:"score"`
	ScoreFmt    string  `json:"-"` // preformatted for templates
}

type SearchResponse struct {
	Query   string `json:"query"`
	TookMs  int64  `json:"took_ms"`
	Total   int    `json:"total"`
	Hits    []Hit  `json:"hits"`
	Engine  string `json:"engine"`
	DocCount int   `json:"doc_count"`
}

type Store struct {
	db     *ultimate_db.DB
	kv     ultimate_db.KVStore
	search *orchid_sync.Engine
	mu     sync.RWMutex
	count  int
}

func NewStore(db *ultimate_db.DB, eng *orchid_sync.Engine) *Store {
	s := &Store{
		db:     db,
		kv:     ultimate_db.NewBTreeKVStore(db),
		search: eng,
	}
	s.count = s.scanDocCount()
	return s
}

func (s *Store) scanDocCount() int {
	n := 0
	txn := s.kv.Begin()
	it := s.kv.NewIterator(txn, []byte(docPrefix))
	defer it.Close()
	for {
		_, _, err := it.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		n++
	}
	return n
}

func (s *Store) DocCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.count
}

func DocIDFromURL(raw string) string {
	u := strings.TrimSpace(strings.ToLower(raw))
	sum := sha1.Sum([]byte(u))
	return hex.EncodeToString(sum[:])
}

func domainOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func (s *Store) putDoc(doc Document) error {
	raw, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	txn := s.kv.Begin()
	return s.kv.Put(txn, []byte(docPrefix+doc.ID), raw, 0)
}

func (s *Store) getDoc(id string) (Document, bool) {
	txn := s.kv.Begin()
	raw, err := s.kv.Get(txn, []byte(docPrefix+id))
	if err != nil || len(raw) == 0 {
		return Document{}, false
	}
	var doc Document
	if json.Unmarshal(raw, &doc) != nil {
		return Document{}, false
	}
	return doc, true
}

// IndexDocument stores the page and updates the BM25 inverted index.
func (s *Store) IndexDocument(doc Document) error {
	doc.URL = strings.TrimSpace(doc.URL)
	doc.Title = strings.TrimSpace(doc.Title)
	doc.Description = strings.TrimSpace(doc.Description)
	doc.Body = strings.TrimSpace(doc.Body)
	if doc.URL == "" {
		return fmt.Errorf("url is required")
	}
	if doc.Title == "" {
		doc.Title = doc.URL
	}
	if doc.ID == "" {
		doc.ID = DocIDFromURL(doc.URL)
	}
	if doc.Domain == "" {
		doc.Domain = domainOf(doc.URL)
	}
	if doc.IndexedAt.IsZero() {
		doc.IndexedAt = time.Now().UTC()
	}

	_, existed := s.getDoc(doc.ID)
	if err := s.putDoc(doc); err != nil {
		return err
	}

	// Index title + description + body for BM25 ranking.
	indexText := strings.Join([]string{
		doc.Title, doc.Title, // boost title
		doc.Description,
		doc.Domain,
		doc.Body,
	}, " ")
	if s.search != nil {
		if err := s.search.Index(doc.ID, indexText); err != nil {
			return err
		}
	}

	if !existed {
		s.mu.Lock()
		s.count++
		s.mu.Unlock()
	}
	return nil
}

// DeleteDocument removes a page from the document store (BM25 postings may linger
// but Search filters missing docs). Idempotent when the URL was never indexed.
func (s *Store) DeleteDocument(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("url is required")
	}
	id := DocIDFromURL(rawURL)
	if _, ok := s.getDoc(id); !ok {
		return nil
	}
	txn := s.kv.Begin()
	if err := s.kv.Delete(txn, []byte(docPrefix+id)); err != nil {
		return err
	}
	s.mu.Lock()
	if s.count > 0 {
		s.count--
	}
	s.mu.Unlock()
	return nil
}

// Search runs BM25 over the corpus. Multi-word queries are OR-joined so bag-of-words ranking applies.
func (s *Store) Search(query string, limit int) (*SearchResponse, error) {
	start := time.Now()
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	query = strings.TrimSpace(query)
	resp := &SearchResponse{
		Query:    query,
		Engine:   "orchid_sync BM25",
		DocCount: s.DocCount(),
		Hits:     []Hit{},
	}
	if query == "" || s.search == nil {
		resp.TookMs = time.Since(start).Milliseconds()
		return resp, nil
	}

	q := normalizeQuery(query)
	results, err := s.search.Search(q, limit*3) // over-fetch then filter missing docs
	if err != nil {
		return nil, err
	}

	terms := orchid_sync.NewAnalyzer().Tokenize(query)
	for _, r := range results {
		doc, ok := s.getDoc(r.DocID)
		if !ok {
			continue
		}
		resp.Hits = append(resp.Hits, Hit{
			ID:          doc.ID,
			URL:         doc.URL,
			Title:       doc.Title,
			Description: doc.Description,
			Domain:      doc.Domain,
			Snippet:     makeSnippet(doc, terms),
			Score:       r.Score,
			ScoreFmt:    fmt.Sprintf("%.4f", r.Score),
		})
		if len(resp.Hits) >= limit {
			break
		}
	}
	resp.Total = len(resp.Hits)
	resp.TookMs = time.Since(start).Milliseconds()
	return resp, nil
}

// normalizeQuery turns free text into an orchid_sync OR query so multi-term BM25 works.
func normalizeQuery(query string) string {
	// Preserve explicit boolean operators.
	if strings.Contains(query, " AND ") || strings.Contains(query, " OR ") || strings.Contains(query, " NOT ") {
		return strings.ToLower(query)
	}
	analyzer := orchid_sync.NewAnalyzer()
	tokens := analyzer.Tokenize(query)
	if len(tokens) == 0 {
		// Fall back to raw lowercased words if everything was stop-words.
		parts := strings.Fields(strings.ToLower(query))
		if len(parts) == 0 {
			return query
		}
		return strings.Join(parts, " OR ")
	}
	if len(tokens) == 1 {
		return tokens[0]
	}
	return strings.Join(tokens, " OR ")
}

var wsCollapse = regexp.MustCompile(`\s+`)

func makeSnippet(doc Document, terms []string) string {
	text := doc.Description
	if text == "" {
		text = doc.Body
	}
	text = wsCollapse.ReplaceAllString(strings.TrimSpace(text), " ")
	if text == "" {
		return ""
	}

	lower := strings.ToLower(text)
	pos := -1
	matchLen := 0
	for _, t := range terms {
		if t == "" {
			continue
		}
		i := strings.Index(lower, strings.ToLower(t))
		if i >= 0 && (pos < 0 || i < pos) {
			pos = i
			matchLen = len(t)
		}
	}

	const maxLen = 180
	if pos < 0 {
		return truncateRunes(text, maxLen)
	}

	start := pos - 40
	if start < 0 {
		start = 0
	}
	// Snap to rune boundary.
	for start > 0 && !utf8.RuneStart(text[start]) {
		start--
	}
	end := pos + matchLen + 120
	if end > len(text) {
		end = len(text)
	}
	for end < len(text) && !utf8.RuneStart(text[end]) {
		end++
	}
	snip := text[start:end]
	if start > 0 {
		snip = "…" + snip
	}
	if end < len(text) {
		snip = snip + "…"
	}
	return snip
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}

// ListRecent returns the most recently indexed documents (scan; fine for seed-sized corpora).
func (s *Store) ListRecent(limit int) []Document {
	if limit <= 0 {
		limit = 20
	}
	var docs []Document
	txn := s.kv.Begin()
	it := s.kv.NewIterator(txn, []byte(docPrefix))
	defer it.Close()
	for {
		_, value, err := it.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		var doc Document
		if json.Unmarshal(value, &doc) != nil {
			continue
		}
		docs = append(docs, doc)
	}
	// Sort by IndexedAt desc (simple insertion for small N).
	for i := 1; i < len(docs); i++ {
		j := i
		for j > 0 && docs[j].IndexedAt.After(docs[j-1].IndexedAt) {
			docs[j], docs[j-1] = docs[j-1], docs[j]
			j--
		}
	}
	if len(docs) > limit {
		docs = docs[:limit]
	}
	return docs
}

// EscapeHTML is used when we need plain text in attributes outside guikit auto-escape paths.
func EscapeHTML(s string) string {
	return html.EscapeString(s)
}

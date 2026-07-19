package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/0TrustCloud/guikit"
	"github.com/0TrustCloud/orchid_log"
	"github.com/0TrustCloud/orchid_sync"
	"github.com/0TrustCloud/product_security"
	"github.com/0TrustCloud/search/otrust"
	"github.com/0TrustCloud/ultimate_db"
)

type App struct {
	DB     *ultimate_db.DB
	GUIKit *guikit.GUIKit
	Store  *Store
	Search *orchid_sync.Engine
	Config Config
	OTrust otrust.Config
}

func main() {
	cfg := LoadConfig()
	ot := cfg.OTrust()
	orch := orchid_log.New(orchid_log.FromEnv("search"))
	orch.Start()
	orch.InstallLogBridge()

	db, err := openDatabase(cfg.DBPath, cfg.WALPath)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}
	defer db.Close()

	// BM25 engine (orchid_sync) — same ranking used by platform log search.
	eng, err := orchid_sync.NewEngine(db, nil, nil)
	if err != nil {
		log.Fatalf("orchid_sync init failed: %v", err)
	}

	index := ultimate_db.NewMemIndex()
	searcher := &ultimate_db.SegmentSearcher{}
	orm := ultimate_db.NewORM(db, index, searcher, cfg.WALPath)
	gk, err := guikit.New(db, orm)
	if err != nil {
		log.Fatalf("GUIKit init failed: %v", err)
	}
	guikit.AppFS = os.DirFS("ui/search")

	app := &App{
		DB:     db,
		GUIKit: gk,
		Store:  NewStore(db, eng),
		Search: eng,
		Config: cfg,
		OTrust: ot,
	}

	if cfg.SeedOnStart && app.Store.DocCount() == 0 {
		app.seedCorpus()
	}

	registerRoutes(app)

	// Machine product indexers POST /api/index with X-Search-Token (no browser Origin).
	var extraOrigins []string
	for _, env := range []string{"SEARCH_ALLOWED_ORIGINS", "OTRUST_ALLOWED_ORIGINS"} {
		for _, part := range strings.Split(os.Getenv(env), ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				extraOrigins = append(extraOrigins, part)
			}
		}
	}
	handler := product_security.Middleware(product_security.Options{
		PublicOrigin: cfg.PublicOrigin,
		ExtraOrigins: extraOrigins,
		ServerToServerPrefixes: []string{
			"/api/webhooks/",
			"/api/index",
		},
	}, otrust.Mount(gk.Mux, ot))
	// Public product: homepage, /search results, and GET /api/search stay open.
	// Only the index console (and session-bearing requests) require DBSC.
	// POST /api/index enforces its own token/session auth.
	handler = product_security.DBSCMiddleware(product_security.DBSCOptions{
		SessionBase:   cfg.AuthURL,
		LoginBeginURL: strings.TrimRight(cfg.PublicOrigin, "/") + "/auth",
		ProtectPrefixes: []string{
			"/index",
			"/admin",
		},
		ExemptPrefixes: []string{
			"/auth", "/guikit.js", "/_0trust/", "/_search/",
			"/api/search", "/api/health", "/api/index", "/health",
		},
		GateAllWithSession: true,
	}, handler)
	if gk != nil {
		gk.SetCheckOrigin(product_security.WSOriginOK(cfg.PublicOrigin, "SEARCH_ALLOWED_ORIGINS"))
	}
	log.Printf("[search] listening on %s (%s; public search; DBSC on /index) · BM25 docs=%d", cfg.ListenAddr, cfg.PublicOrigin, app.Store.DocCount())
	orch.Boot(cfg.ListenAddr)
	server := product_security.SecureServer(cfg.ListenAddr, handler)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func registerRoutes(app *App) {
	gk := app.GUIKit

	gk.Get("/logout", func(c *guikit.Context) {
		http.SetCookie(c.W, &http.Cookie{Name: "session_id", MaxAge: -1, Path: "/"})
		http.Redirect(c.W, c.R, app.OTrust.LogoutURL(), http.StatusSeeOther)
	})

	// Public search UI
	gk.Get("/", app.handleHome)
	gk.Get("/search", app.handleSearch)
	gk.Get("/about", app.handleAbout)

	// Index console (auth)
	gk.Get("/index", RequireAuth(app, app.handleIndexPage))
	gk.Post("/index", RequireAuth(app, app.handleIndexSubmit))

	// APIs
	gk.Get("/api/search", app.apiSearch)
	gk.Post("/api/index", app.apiIndex)
	gk.Get("/api/health", app.apiHealth)
	gk.Get("/_search/health", app.apiHealth)

	// JSON convenience on health under guikit path
	gk.Get("/health", func(c *guikit.Context) {
		c.W.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(c.W).Encode(map[string]interface{}{
			"status": "ok",
			"docs":   app.Store.DocCount(),
			"engine": "orchid_sync BM25",
		})
	})
}

func RequireAuth(app *App, next func(c *guikit.Context)) func(c *guikit.Context) {
	return func(c *guikit.Context) {
		user, ok := app.OTrust.RequireSession(c.W, c.R)
		if !ok {
			return
		}
		c.Data["CurrentUser"] = user
		c.Data["SignedIn"] = true
		next(c)
	}
}

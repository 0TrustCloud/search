package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/0TrustCloud/guikit"
)

func (app *App) baseData(c *guikit.Context) {
	c.Data["PublicURL"] = app.Config.PublicOrigin
	c.Data["AuthURL"] = app.Config.AuthURL
	c.Data["DocCount"] = app.Store.DocCount()
	c.Data["Engine"] = "orchid_sync BM25"
	c.Data["Brand"] = "0Trust Search"
	if user, ok := app.optionalUser(c.R); ok {
		c.Data["CurrentUser"] = user
		c.Data["SignedIn"] = true
	} else {
		c.Data["SignedIn"] = false
	}
}

func (app *App) optionalUser(r *http.Request) (string, bool) {
	st, code := app.OTrust.SessionStatus(r)
	if code != http.StatusOK || !st.Valid {
		return "", false
	}
	if st.Subject != "" {
		return st.Subject, true
	}
	return "", false
}

func (app *App) handleHome(c *guikit.Context) {
	app.baseData(c)
	q := strings.TrimSpace(c.R.URL.Query().Get("q"))
	if q != "" {
		http.Redirect(c.W, c.R, "/search?q="+url.QueryEscape(q), http.StatusSeeOther)
		return
	}
	app.GUIKit.Render(c, "home")
}

func (app *App) handleSearch(c *guikit.Context) {
	app.baseData(c)
	q := strings.TrimSpace(c.R.URL.Query().Get("q"))
	page := 1
	if p := c.R.URL.Query().Get("p"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	limit := 10
	c.Data["Query"] = q
	c.Data["Page"] = page

	if q == "" {
		c.Data["EmptyQuery"] = true
		c.Data["Hits"] = []Hit{}
		c.Data["Total"] = 0
		c.Data["TookMs"] = int64(0)
		app.GUIKit.Render(c, "results")
		return
	}

	// Simple offset: fetch page*limit then slice (fine for seed-scale corpus).
	fetch := page * limit
	if fetch > 50 {
		fetch = 50
	}
	resp, err := app.Store.Search(q, fetch)
	if err != nil {
		c.Data["Error"] = err.Error()
		c.Data["Hits"] = []Hit{}
		c.Data["Total"] = 0
		c.Data["TookMs"] = int64(0)
		app.GUIKit.Render(c, "results")
		return
	}

	start := (page - 1) * limit
	hits := resp.Hits
	if start >= len(hits) {
		hits = []Hit{}
	} else {
		end := start + limit
		if end > len(hits) {
			end = len(hits)
		}
		hits = hits[start:end]
	}

	c.Data["Hits"] = hits
	c.Data["Total"] = resp.Total
	c.Data["TookMs"] = resp.TookMs
	c.Data["HasMore"] = len(resp.Hits) >= page*limit && page*limit < 50
	c.Data["NextPage"] = page + 1
	c.Data["PrevPage"] = page - 1
	c.Data["HasPrev"] = page > 1
	c.Data["HasResults"] = resp.Total > 0
	// Prebuild pager URLs so GML templates stay simple.
	if page > 1 {
		c.Data["PrevURL"] = "/search?q=" + url.QueryEscape(q) + "&p=" + strconv.Itoa(page-1)
	}
	if len(resp.Hits) >= page*limit && page*limit < 50 {
		c.Data["NextURL"] = "/search?q=" + url.QueryEscape(q) + "&p=" + strconv.Itoa(page+1)
	}
	app.GUIKit.Render(c, "results")
}

func (app *App) handleAbout(c *guikit.Context) {
	app.baseData(c)
	app.GUIKit.Render(c, "about")
}

func (app *App) handleIndexPage(c *guikit.Context) {
	app.baseData(c)
	c.Data["Recent"] = app.Store.ListRecent(25)
	if msg := c.R.URL.Query().Get("msg"); msg != "" {
		c.Data["Flash"] = msg
	}
	app.GUIKit.Render(c, "index")
}

func (app *App) handleIndexSubmit(c *guikit.Context) {
	_ = c.R.ParseForm()
	rawURL := strings.TrimSpace(c.R.FormValue("url"))
	title := strings.TrimSpace(c.R.FormValue("title"))
	desc := strings.TrimSpace(c.R.FormValue("description"))
	body := strings.TrimSpace(c.R.FormValue("body"))
	fetch := c.R.FormValue("fetch") == "1"

	if rawURL == "" {
		http.Redirect(c.W, c.R, "/index?msg="+url.QueryEscape("URL is required"), http.StatusSeeOther)
		return
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	doc := Document{
		URL:         rawURL,
		Title:       title,
		Description: desc,
		Body:        body,
		Source:      "submit",
	}
	if user, ok := c.Data["CurrentUser"].(string); ok && user != "" {
		doc.Source = "submit:" + user
	}

	if fetch || (body == "" && title == "") {
		fetched, err := fetchURL(rawURL)
		if err != nil {
			if body == "" && title == "" {
				http.Redirect(c.W, c.R, "/index?msg="+url.QueryEscape("fetch failed: "+err.Error()), http.StatusSeeOther)
				return
			}
		} else {
			if doc.Title == "" {
				doc.Title = fetched.Title
			}
			if doc.Description == "" {
				doc.Description = fetched.Description
			}
			if doc.Body == "" {
				doc.Body = fetched.Body
			}
			doc.Source = "crawl"
		}
	}

	if err := app.Store.IndexDocument(doc); err != nil {
		http.Redirect(c.W, c.R, "/index?msg="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(c.W, c.R, "/index?msg="+url.QueryEscape("Indexed "+rawURL), http.StatusSeeOther)
}

func (app *App) apiSearch(c *guikit.Context) {
	q := strings.TrimSpace(c.R.URL.Query().Get("q"))
	limit, _ := strconv.Atoi(c.R.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}
	resp, err := app.Store.Search(q, limit)
	c.W.Header().Set("Content-Type", "application/json")
	if err != nil {
		c.W.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(c.W).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(c.W).Encode(resp)
}

func (app *App) apiIndex(c *guikit.Context) {
	// Allow session user OR shared index token.
	tokenOK := app.Config.IndexToken != "" &&
		(c.R.Header.Get("X-Search-Token") == app.Config.IndexToken ||
			c.R.URL.Query().Get("token") == app.Config.IndexToken)
	if !tokenOK {
		if _, ok := app.OTrust.RequireSession(c.W, c.R); !ok {
			return
		}
	}

	var in struct {
		URL         string `json:"url"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Body        string `json:"body"`
		Fetch       bool   `json:"fetch"`
	}
	body, _ := io.ReadAll(io.LimitReader(c.R.Body, 1<<20))
	if len(body) > 0 {
		if err := json.Unmarshal(body, &in); err != nil {
			http.Error(c.W, "invalid json", http.StatusBadRequest)
			return
		}
	} else {
		_ = c.R.ParseForm()
		in.URL = c.R.FormValue("url")
		in.Title = c.R.FormValue("title")
		in.Description = c.R.FormValue("description")
		in.Body = c.R.FormValue("body")
		in.Fetch = c.R.FormValue("fetch") == "1"
	}

	if strings.TrimSpace(in.URL) == "" {
		http.Error(c.W, "url required", http.StatusBadRequest)
		return
	}
	rawURL := strings.TrimSpace(in.URL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	doc := Document{
		URL:         rawURL,
		Title:       in.Title,
		Description: in.Description,
		Body:        in.Body,
		Source:      "api",
	}
	if in.Fetch || (doc.Body == "" && doc.Title == "") {
		if fetched, err := fetchURL(rawURL); err == nil {
			if doc.Title == "" {
				doc.Title = fetched.Title
			}
			if doc.Description == "" {
				doc.Description = fetched.Description
			}
			if doc.Body == "" {
				doc.Body = fetched.Body
			}
			doc.Source = "crawl"
		}
	}

	if err := app.Store.IndexDocument(doc); err != nil {
		http.Error(c.W, err.Error(), http.StatusBadRequest)
		return
	}
	c.W.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(c.W).Encode(map[string]interface{}{
		"ok":  true,
		"id":  DocIDFromURL(rawURL),
		"url": rawURL,
	})
}

func (app *App) apiHealth(c *guikit.Context) {
	c.W.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(c.W).Encode(map[string]interface{}{
		"status":    "ok",
		"service":   "search",
		"engine":    "orchid_sync BM25",
		"docs":      app.Store.DocCount(),
		"public":    app.Config.PublicOrigin,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// --- lightweight HTML fetch for optional crawl ---

type fetchedPage struct {
	Title       string
	Description string
	Body        string
}

var (
	reTitle       = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reMetaDesc    = regexp.MustCompile(`(?is)<meta[^>]+name=["']description["'][^>]+content=["']([^"']*)["']`)
	reMetaDesc2   = regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']*)["'][^>]+name=["']description["']`)
	reScript = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reNoscript = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	reTags   = regexp.MustCompile(`(?s)<[^>]+>`)
	reEntities    = strings.NewReplacer(
		"&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'",
	)
)

func fetchURL(raw string) (*fetchedPage, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("only http(s) supported")
	}

	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "0TrustSearchBot/1.0 (+https://search.0trust.cloud)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(strings.ToLower(ct), "html") && !strings.Contains(strings.ToLower(ct), "text/") {
		return nil, fmt.Errorf("unsupported content-type %s", ct)
	}

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	html := string(rawBody)

	out := &fetchedPage{}
	if m := reTitle.FindStringSubmatch(html); len(m) > 1 {
		out.Title = cleanText(m[1])
	}
	if m := reMetaDesc.FindStringSubmatch(html); len(m) > 1 {
		out.Description = cleanText(m[1])
	} else if m := reMetaDesc2.FindStringSubmatch(html); len(m) > 1 {
		out.Description = cleanText(m[1])
	}

	stripped := reScript.ReplaceAllString(html, " ")
	stripped = reStyle.ReplaceAllString(stripped, " ")
	stripped = reNoscript.ReplaceAllString(stripped, " ")
	stripped = reTags.ReplaceAllString(stripped, " ")
	out.Body = cleanText(stripped)
	if len(out.Body) > 20000 {
		out.Body = out.Body[:20000]
	}
	if out.Title == "" {
		out.Title = u.Hostname()
	}
	return out, nil
}

func cleanText(s string) string {
	s = reEntities.Replace(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if r == '\u0000' {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

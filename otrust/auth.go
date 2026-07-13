package otrust

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/0TrustCloud/product_otrust"
	"github.com/0TrustCloud/product_security"
)


func trustedProxyRequest(r *http.Request) bool {
	return product_security.TrustedProxyRequest(r, "OTRUST_TRUSTED_PROXIES")
}

type Config struct {
	AuthURL      string // identity plane, e.g. https://search.0trust.cloud or dedicated IdP
	PublicOrigin string // product site, e.g. https://search.0trust.cloud
	ClientID     string
	ClientSecret string
	RedirectURI  string
	ServiceName  string
}

type SessionStatus struct {
	Subject       string `json:"sub"`
	Valid         bool   `json:"valid"`
	DBSCBound     bool   `json:"dbsc_bound"`
	HardwareBound bool   `json:"hardware_bound"`
}

func (c Config) authURL() string {
	u := strings.TrimRight(strings.TrimSpace(c.AuthURL), "/")
	if u == "" {
		return "https://search.0trust.cloud"
	}
	return u
}

func (c Config) publicOrigin() string {
	u := strings.TrimRight(strings.TrimSpace(c.PublicOrigin), "/")
	if u == "" {
		return "https://search.0trust.cloud"
	}
	return u
}

func (c Config) authHost() string {
	u, _ := url.Parse(c.authURL())
	if u == nil || u.Host == "" {
		return "search.0trust.cloud"
	}
	h := u.Host
	if host, _, err := net.SplitHostPort(h); err == nil {
		return host
	}
	return h
}

func (c Config) serviceName() string {
	if strings.TrimSpace(c.ServiceName) != "" {
		return c.ServiceName
	}
	return "search"
}

func (c Config) dualAuth() product_otrust.DualAuth {
	return product_otrust.DualAuthFromEnv(c.publicOrigin(), c.domainAuthURL(), "search.0trust.cloud")
}

func (c Config) DualAuth() product_otrust.DualAuth { return c.dualAuth() }

func (c Config) domainAuthURL() string { return c.authURL() }

func (c Config) identityURL(r *http.Request) string { return c.dualAuth().IdentityURL(r) }

// Mount wraps the app; auth lives on the selected IdP — callback and SAMLn consume stay on the product host.
func Mount(next http.Handler, cfg Config) http.Handler {
	onSession := func(w http.ResponseWriter, r *http.Request, sessionVal, dest string) {
		setSessionCookie(w, r, sessionVal)
		http.Redirect(w, r, dest, http.StatusSeeOther)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/auth/callback" {
			handleCallback(w, r, cfg)
			return
		}
		if path == "/auth/samln/consume" {
			product_otrust.SAMLnConsumeForRequest(w, r, cfg.dualAuth(), cfg.publicOrigin(), cfg.ClientID, cfg.RedirectURI, onSession)
			return
		}
		if path == "/auth/mode" {
			product_otrust.HandleAuthMode(w, r, cfg.dualAuth())
			return
		}
		if path == "/auth" {
			if product_otrust.HandleAuthGate(w, r, cfg.dualAuth()) {
				return
			}
		}
		if path == "/auth/logout" {
			product_otrust.HandleLogout(w, r, cfg.identityURL(r), cfg.publicOrigin())
			return
		}
		if product_otrust.IsProxiedAuthPath(path) {
			if product_otrust.IsProxiedAuthAsset(path) {
				product_otrust.ProxyIdentityAsset(w, r, cfg.identityURL(r))
				return
			}
			product_otrust.RedirectToIdentity(w, r, cfg.identityURL(r), cfg.publicOrigin())
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (c Config) LogoutURL() string {
	return product_otrust.LogoutURL(c.publicOrigin())
}

func (c Config) AuthEntryURL(returnTo string) string {
	return product_otrust.EntryURL(c.publicOrigin(), returnTo)
}

func handleCallback(w http.ResponseWriter, r *http.Request, cfg Config) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, cfg.AuthEntryURL(cfg.publicOrigin()+"/"), http.StatusSeeOther)
		return
	}

	tokenURL := cfg.identityURL(r) + "/auth/token"
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", cfg.RedirectURI)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)

	req, _ := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = "token exchange rejected"
		}
		http.Error(w, msg, resp.StatusCode)
		return
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	_ = json.Unmarshal(body, &tok)

	sessionVal := firstNonEmpty(tok.AccessToken, tok.IDToken, "ok")
	setSessionCookie(w, r, sessionVal)

	dest := r.URL.Query().Get("return_to")
	if dest == "" {
		dest = "/"
	}
	if !strings.HasPrefix(dest, "/") {
		if u, err := url.Parse(dest); err == nil && u.Host == "" {
			dest = u.Path
		} else if strings.HasPrefix(dest, cfg.publicOrigin()) {
			// full URL on our product origin
		} else {
			dest = "/"
		}
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, sessionVal string) {
	secure := r != nil && (r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"))
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionVal,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (c Config) SessionStatus(r *http.Request) (SessionStatus, int) {
	// Only trust mesh/0trust.services edge headers from loopback or OTRUST_TRUSTED_PROXIES.
	// Mesh / 0trust.services edge: only honor subject when DBSC was verified upstream
	// (X-0Trust-DBSC: bound set by requireDBSCSession on the access plane).
	if trustedProxyRequest(r) {
		if sub := strings.TrimSpace(r.Header.Get("X-0Trust-Subject")); sub != "" && r.Header.Get("X-0Trust-DBSC") == "bound" {
			return SessionStatus{
				Subject:       sub,
				Valid:         true,
				DBSCBound:     true,
				HardwareBound: true,
			}, http.StatusOK
		}
	}

	cookie, err := r.Cookie("session_id")
	if err != nil {
		return SessionStatus{}, http.StatusUnauthorized
	}

	req, err := http.NewRequest(http.MethodGet, c.identityURL(r)+"/api/v1/idp/session", nil)
	if err != nil {
		return SessionStatus{}, http.StatusBadGateway
	}
	req.AddCookie(cookie)
	for _, h := range []string{"Sec-Session-Response", "Sec-Session-Challenge", "Sec-Session-Id"} {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return SessionStatus{}, http.StatusBadGateway
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return SessionStatus{}, resp.StatusCode
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var st SessionStatus
	if json.Unmarshal(body, &st) != nil {
		return SessionStatus{}, http.StatusBadGateway
	}
	return st, http.StatusOK
}

func (c Config) RequireSession(w http.ResponseWriter, r *http.Request) (string, bool) {
	st, code := c.SessionStatus(r)
	if code == http.StatusUnauthorized || !st.Valid {
		returnTo := c.publicOrigin() + r.URL.RequestURI()
		http.Redirect(w, r, c.AuthEntryURL(returnTo), http.StatusSeeOther)
		return "", false
	}
	if !st.DBSCBound && !st.HardwareBound {
		returnTo := c.publicOrigin() + r.URL.RequestURI()
		http.Redirect(w, r, c.identityURL(r)+"/auth/login/begin?dbsc=required&return_to="+url.QueryEscape(returnTo), http.StatusFound)
		return "", false
	}
	subject := strings.TrimSpace(st.Subject)
	if subject == "" || len(subject) > 128 {
		subject = ""
	}
	if subject == "" {
		returnTo := c.publicOrigin() + r.URL.RequestURI()
		http.Redirect(w, r, c.AuthEntryURL(returnTo), http.StatusSeeOther)
		return "", false
	}
	return subject, true
}

func sessionSubjectFromCookie(r *http.Request) string {
	if c, err := r.Cookie("session_id"); err == nil && c.Value != "" {
		user := strings.TrimPrefix(c.Value, "login_")
		user = strings.TrimPrefix(user, "reg_")
		if user != "" {
			return user
		}
	}
	return ""
}

func ConfigFromEnv() Config {
	public := strings.TrimRight(os.Getenv("SEARCH_PUBLIC_URL"), "/")
	if public == "" {
		public = "https://search.0trust.cloud"
	}
	auth := strings.TrimRight(os.Getenv("SEARCH_AUTH_URL"), "/")
	if auth == "" {
		auth = public
	}
	if idp := strings.TrimRight(os.Getenv("SEARCH_IDP_URL"), "/"); idp != "" {
		auth = idp
	}
	redirect := os.Getenv("OTRUST_REDIRECT_URI")
	if redirect == "" {
		redirect = public + "/auth/callback"
	}
	return Config{
		AuthURL:      auth,
		PublicOrigin: public,
		ClientID:     os.Getenv("OTRUST_CLIENT_ID"),
		ClientSecret: os.Getenv("OTRUST_CLIENT_SECRET"),
		RedirectURI:  redirect,
		ServiceName:  "search",
	}
}

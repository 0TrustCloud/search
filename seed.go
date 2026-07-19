package main

import "log"

// seedCorpus indexes a bootstrap set of 0Trust ecosystem pages so search is useful on first boot.
func (app *App) seedCorpus() {
	docs := []Document{
		{
			URL:         "https://0trust.cloud",
			Title:       "0Trust.Cloud — Zero-trust control plane",
			Description: "Integrated identity, ZTNA access, observability, and WAN deploy. OIDC, WebAuthn passkeys, DBSC device-bound sessions.",
			Body:        "0Trust.Cloud combines OIDC and WebAuthn identity, device-bound session credentials (DBSC), ZTNA HTTP proxying, Orchid-compatible logging with BM25 search, and one-command WAN deploy. Protect a process on localhost and expose it with policy-gated access without a traditional VPN. Platform dashboard, Teleport-style app access, Elastic-style log explorer, and service agent tunnels to 0trust.services.",
			Source:      "seed",
		},
		{
			URL:         "https://0trust.cloud/dashboard",
			Title:       "0Trust Dashboard",
			Description: "Service hub for IdP, Teleport apps, Elastic logs, and local app deploy.",
			Body:        "Configure OIDC clients, register ZTNA applications, search platform logs with BM25, provision local apps to subdomains on 0trust.services, manage service keys, PKI, nameservice, and workflows.",
			Source:      "seed",
		},
		{
			URL:         "https://0trust.services",
			Title:       "0Trust.Services — ngrok-style WAN tunnels",
			Description: "Expose localhost apps on {sub}.0trust.services with identity and audit.",
			Body:        "0Trust Services agent tunnels a local HTTP app to a public HTTPS URL. QUIC control channel, OIDC and WebAuthn auth proxy, DBSC-bound sessions before app traffic. Deploy with services.exe --subdomain myapp --port 3000.",
			Source:      "seed",
		},
		{
			URL:         "https://williwaw.app",
			Title:       "Williwaw — Social on the 0Trust mesh",
			Description: "Stories, votes, follows, RSS feeds, and moderation on williwaw.app and .social.",
			Body:        "Williwaw is a social news and discussion product. Post stories, vote, follow users, import RSS, track karma, and moderate content. Identity via williwaw.0trust.cloud passkeys. Private TLD williwaw.social resolves on the 0Trust mesh DNS plane.",
			Source:      "seed",
		},
		{
			URL:         "https://tunneltug.com",
			Title:       "TunnelTug — QUIC tunnels for localhost",
			Description: "Load-balanced QUIC control tunnels, HTTP/3 ingress, mesh and vhosts.",
			Body:        "TunnelTug tugs your localhost to the internet over QUIC. Yamux multiplexed streams, load-balanced barge fleets, subdomain routing on tunneltug.com, mesh registration as .tunnel names, ACME TLS, and 0Trust passkey dashboard.",
			Source:      "seed",
		},
		{
			URL:         "https://defcon.chat",
			Title:       "Ack / defcon.chat — Secure team chat",
			Description: "Realtime chat with 0Trust identity, channels, GIFs, and moderation.",
			Body:        "Ack powers defcon.chat. Channels, realtime hub, webhooks, GIF search, permissions, and passkey login through defcon.0trust.cloud. Built with guikit and product otrust.",
			Source:      "seed",
		},
		{
			URL:         "https://bandy.chat",
			Title:       "Bandy — Chat for the mesh",
			Description: "Realtime messaging product with 0Trust auth and moderation tools.",
			Body:        "Bandy is a chat application sibling of Ack. Realtime messages, channels, webhooks, GIF support, and OIDC/WebAuthn identity on bandy.0trust.cloud.",
			Source:      "seed",
		},
		{
			URL:         "https://motionkb.com",
			Title:       "MotionKB — Docs and sites",
			Description: "Documentation CMS and public sites on MotionKB and 0trust.cloud.",
			Body:        "MotionKB is a documentation and site builder. Pages, blocks, public docs search, edge host sync, and PWA support. Editors authenticate with MotionKB IdP; public docs are open on 0trust.cloud.",
			Source:      "seed",
		},
		{
			URL:         "https://0trust.name",
			Title:       "0Trust Name — Private DNS and gTLD face",
			Description: "Registrar face for private TLDs .mesh .social .tunnel with DoH split DNS.",
			Body:        "0Trust Name operates private extension zones. Authoritative NS at ns.0trust.cloud and ns.0trust.services. DNS-over-HTTPS at dns.0trust.name and dns.0trust.cloud. Split resolver: mesh private TLDs locally, everything else to public resolvers.",
			Source:      "seed",
		},
		{
			URL:         "https://search.0trust.cloud",
			Title:       "0Trust Search — BM25 web search",
			Description: "Google-style search powered by orchid_sync Okapi BM25 on the 0Trust mesh.",
			Body:        "0Trust Search is a full-text search engine using the Okapi BM25 ranking algorithm from orchid_sync. Index web pages by URL, search with natural language queries, and browse ranked results with title, URL, and snippet. Built with guikit. Hosted at search.0trust.cloud.",
			Source:      "seed",
		},
		{
			URL:         "https://0trust.cloud/logs",
			Title:       "Orchid / Elastic log search",
			Description: "Platform log explorer with BM25 full-text search and live tail.",
			Body:        "Platform observability uses orchid_sync BM25 for log search. Ingest via API key, BM25 ranked results, retention policies, cold storage, and distributed scatter-gather across mesh peers when enabled.",
			Source:      "seed",
		},
		{
			URL:         "https://0trust.social",
			Title:       "0Trust Social CDN",
			Description: "Media and social CDN plane for mesh products.",
			Body:        "0Trust Social hosts media blobs, range requests, moderation hooks, and CDN delivery used by Williwaw, Ack, Bandy, and other products. JWT-backed access and product identity integration.",
			Source:      "seed",
		},
		{
			URL:         "https://github.com/orgs/0TrustCloud/repositories",
			Title:       "0TrustCloud on GitHub",
			Description: "Open module ecosystem: guikit, ultimate_db, orchid_sync, secure_network, and more.",
			Body:        "0TrustCloud publishes reusable Go modules including guikit reactive UI, ultimate_db embedded database, orchid_sync BM25 search, secure_network mesh, auth_provider WebAuthn and DBSC, service keys, and secure DNS. Build zero-trust products on the same stack.",
			Source:      "seed",
		},
		{
			URL:         "https://0trust.cloud/access",
			Title:       "0Trust ZTNA — Teleport-style app access",
			Description: "Policy-gated HTTP proxy to internal applications at /access/{app}.",
			Body:        "Zero trust network access proxies HTTP applications with allowed_roles and policy checks on every request. Register apps in the dashboard, reach them at 0trust.cloud/access/appid without a VPN. Sessions require passkey login and DBSC binding.",
			Source:      "seed",
		},
		{
			URL:         "https://mail.0trust.cloud",
			Title:       "0Trust Mail",
			Description: "Mesh-aware mail gateway with SMTP, DKIM, and web mailbox UI.",
			Body:        "0Trust Mail provides local mailboxes, SMTP receive and delivery, DKIM signing, templates, and a guikit web UI. Integrated with product otrust identity on mail.0trust.cloud.",
			Source:      "seed",
		},
	}

	indexed := 0
	for _, d := range docs {
		if err := app.Store.IndexDocument(d); err != nil {
			log.Printf("[search] seed index %s: %v", d.URL, err)
			continue
		}
		indexed++
	}
	log.Printf("[search] seeded %d documents (corpus size %d)", indexed, app.Store.DocCount())
}

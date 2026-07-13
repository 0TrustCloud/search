# 0Trust Search (`search.0trust.cloud`)

Google-style web search for the 0Trust mesh, ranked with **orchid_sync Okapi BM25** and rendered with **guikit**.

## Features

- Homepage + results UI (title, URL, snippet, BM25 score)
- Inverted index via `orchid_sync` (same BM25 as platform log search)
- Document store in `ultimate_db` (URL, title, description, body)
- Seed corpus of 0Trust ecosystem pages on first boot
- Optional HTML fetch when indexing a URL
- JSON API: `GET /api/search`, `POST /api/index`, `GET /api/health`
- Passkey index console at `/index` (product otrust)

## Run locally

```powershell
# from repo root
go build -o bin\search.exe .\cmd\search
$env:SEARCH_LISTEN=":3087"
$env:SEARCH_PUBLIC_URL="http://127.0.0.1:3087"
$env:SEARCH_DB="data/search.db"
$env:SEARCH_WAL="data/search.wal"
.\bin\search.exe
```

Open `http://127.0.0.1:3087` — try queries like `williwaw`, `bm25`, `tunnel`.

## Environment

| Variable | Default | Purpose |
|----------|---------|---------|
| `SEARCH_LISTEN` | `:3087` | Listen address |
| `SEARCH_PUBLIC_URL` | `https://search.0trust.cloud` | Public origin |
| `SEARCH_IDP_URL` | (optional) | Identity plane base URL |
| `SEARCH_DB` / `SEARCH_WAL` | `data/search.db` / `.wal` | Persistence |
| `SEARCH_SEED` | `true` | Seed ecosystem docs when empty |
| `SEARCH_INDEX_TOKEN` | empty | Machine token for `POST /api/index` |

## Production

- Edge vhost: `search.0trust.cloud` → `127.0.0.1:3087` (`config/services-server.yaml`)
- Unit: `deploy/droplet/search.service`
- Env example: `config/search.env.example`

## API examples

```http
GET /api/search?q=zero+trust&limit=10

POST /api/index
Content-Type: application/json
X-Search-Token: <token>

{"url":"https://example.com","fetch":true}
```

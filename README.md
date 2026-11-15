# torrent-indexer

[![](https://dcbadge.limes.pink/api/server/7wqNywmpQW)](https://discord.gg/7wqNywmpQW)
[![Sponsor](https://img.shields.io/badge/Sponsor-❤-ff69b4?style=for-the-badge&logo=github)](https://github.com/sponsors/felipemarinho97)

This is a simple torrent indexer that can be used to index torrents from HTML pages. It is written in Golang and uses Redis as a cache.

## Test it

Visit [https://torrent-indexer.darklyn.org/](https://torrent-indexer.darklyn.org/) to test it.

## Supported sites

- [comando-torrents](https://comando1.com/)
- [bludv](https://bludvfilmes2.xyz/)
- [torrent-dos-filmes](https://torrentdosfilmes-v2.xyz/)
- [starck-filmes](https://starckfilmes-v28.com/)
- [rede-torrent](https://redestorrents.com/)
- [vaca-torrent](https://vaqueirofilmes.com/)

## Deploy

If you have Docker + docker-compose installed, you can deploy it using the following command:

```bash
curl -s https://raw.githubusercontent.com/felipemarinho97/torrent-indexer/main/docker-compose.yml > docker-compose.yml
docker-compose up -d
```

The server will be available at [http://localhost:8080/](http://localhost:8080/).

## Configuration

You can configure the server using the following environment variables:
  
- `PORT`: (optional) The port that the server will listen to. Default: `7006`
- `HOST`: (optional) Bind address. Default: `127.0.0.1`; set `0.0.0.0` to allow LAN access.
- `METRICS_PORT`: (optional) The port that the metrics server will listen to. Default: `9081`
- `LOG_LEVEL`: (optional) The log level. Default: `1` (info). 
  - Possible values: `0` (debug), `1` (info), `2` (warn), `3` (error)
- `LOG_FORMAT`: (optional) The log format. Can be "json" or default to console logger.
- `FLARESOLVERR_ADDRESS`: (optional) The address of the FlareSolverr instance. Default: `N/A`
- `FLARESOLVERR_TIMEOUT_SECONDS`: (optional) Timeout for flaresolverr requests. Default: `30`
- `FLARESOLVERR_POOL_SIZE`: (optional) Number of FlareSolverr sessions in the pool. Default: `5`
- `FLARESOLVERR_USE_SESSIONS`: (optional) Reuse browser sessions. Default: `false`; sessionless mode also works with FlareSolverr-Go.
- `REQUEST_TIMEOUT_MILLISECONDS`: (optional) Timeout for external scraping requests. Default: `5000`
- `MEILISEARCH_ADDRESS`: (optional) The address of the MeiliSearch instance. Default: `N/A`
- `MEILISEARCH_KEY`: (optional) The API key of the MeiliSearch instance. Default: `N/A`
- `TMDB_API_KEY`: (optional) TMDB v3 API key. When set, newly indexed torrents are enriched in the background with overview, poster, genres, runtime, rating and TMDB link. Metadata is cached in Redis. Default: disabled.
- `REDIS_HOST`: (optional) The address of the Redis instance. Default: `localhost`
- `REDIS_PASSWORD`: (optional) The password of the Redis instance. Default: `N/A`
- `SHORT_LIVED_CACHE_EXPIRATION` (optional) The expiration time of the short-lived cache in duration format. Default: `30m`
    - This cache is used to cache homepage or search results.
    - Example: `30m`, `1h`, `1h30m`, `1h30m30s`
- `LONG_LIVED_CACHE_EXPIRATION` (optional) The expiration time of the long-lived cache in duration format. Default: `7d`
    - This cache is used to store the torrent webpages (posts). You can set it to a higher value because the torrent pages are not updated frequently.

### Extra Configuration
- `FALLBACK_TITLE_ENABLED`: (optional) Enable the fallback title post-processor that sets the title to "[UNSAFE] {original_title}" (Page title) if the title is empty. Default: `false`
    - This is useful for sites that do not have a title for some torrents, but can lead to misleading titles.
- `MAGNET_METADATA_API_ENABLED`: (optional) Enable the magnet metadata API. (deploy instrucitons [here](https://github.com/felipemarinho97/magnet-metadata-api)) Default: `false`
- `MAGNET_METADATA_API_ADDRESS`: (optional) The address of your magnet metadata API. Default: `N/A`
- `MAGNET_METADATA_API_TIMEOUT_SECONDS`: (optional) The timeout for the magnet metadata API requests in seconds. Default: `10`
- `INDEXER_<NAME>_URL`: (optional) Set a custom URL for the indexer. Where the "NAME" will be always uppercase indexer key with underscores. ex: `INDEXER_DODO_FILMES_URL=https://my-proxied-dodo-url.org`

### Operations dashboard

Open `/ops` to view service health, Redis and Meilisearch status, TMDB configuration, and recent request counts, errors, status and duration for each indexer. The dashboard refreshes every 15 seconds. Request counts are in-memory and reset when the process restarts. The JSON snapshot is available at `/ops/status`.

### TMDB metadata

Set `TMDB_API_KEY` to enable metadata enrichment. Existing IMDb IDs are used first; titles and years are used as a fallback. Enrichment is cached in Redis and does not delay indexer responses. Search requests also make a bounded enrichment attempt for existing records, then save successful matches to Meilisearch. The browser search page displays the poster, synopsis, genres, runtime, rating and TMDB link when available.

TMDB metadata and images are provided under TMDB's API terms. The interface includes TMDB attribution: “This product uses the TMDB API but is not endorsed or certified by TMDB.”

    | Endpoint (`/indexers/...`) | Variable | Default |
    |---|---|---|
    | `comando_torrents` | `INDEXER_COMANDO_URL` | `https://comando1.com/` |
    | `bludv` | `INDEXER_BLUDV_URL` | `https://bludvfilmes2.xyz/` |
    | `torrent-dos-filmes` | `INDEXER_TORRENT_DOS_FILMES_URL` | `https://torrentdosfilmes-v2.xyz/` |
    | `starck-filmes` | `INDEXER_STARCK_FILMES_URL` | `https://starckfilmes-v28.com/` |
    | `rede_torrent` | `INDEXER_REDE_TORRENT_URL` | `https://redestorrents.com/` |
    | `vaca_torrent` | `INDEXER_VACA_TORRENT_URL` | `https://vaqueirofilmes.com/` |

## Integrating with Jackett

You can integrate this indexer with Jackett by adding a new Torznab custom indexer. Here is an example of how to do it for the `bludv` indexer:

```yaml
---
id: bludv_indexer
name: BluDV Indexer
description: "BluDV - Custom indexer on from torrent-indexer"
language: pt-BR
type: public
encoding: UTF-8
links:
  - http://localhost:8080/

caps:
  categorymappings:
    - { id: Movie, cat: Movies, desc: "Movies" }
    - { id: TV, cat: TV, desc: "TV" }

  modes:
    search: [q]
    tv-search: [q, season, ep]
    movie-search: [q]
  allowrawsearch: true

settings: []

search:
  paths:
    - path: "indexers/bludv?filter_results=true&q={{ .Keywords }}"
      response:
        type: json

  keywordsfilters:
    - name: tolower

  rows:
    selector: $.results
    count:
      selector: $.count

  fields:
    _id:
      selector: title
    download:
      selector: magnet_link
    title:
      selector: title
    description:
      selector: original_title
    details:
      selector: details
    infohash:
      selector: info_hash
    date:
      selector: date
    size:
      selector: size
    seeders:
      selector: seed_count
    leechers:
      selector: leech_count
    imdb:
      selector: imdb
    category_is_tv_show:
      selector: title
      filters:
        - name: regexp
          args: "\\b(S\\d+(?:E\\d+)?)\\b"
    category:
      text: "{{ if .Result.category_is_tv_show }}TV{{ else }}Movie{{ end }}"
# json engine n/a
```

## Integrating with Prowlarr

The server exposes a native Torznab API. In Prowlarr, add a **Generic Torznab** indexer for each source. Use the matching API path:

| Source | API path |
|---|---|
| BluDV | `/api/bludv` |
| Comando Torrents | `/api/comando_torrents` |
| Rede Torrent | `/api/rede_torrent` |
| Starck Filmes | `/api/starck-filmes` |
| Torrent dos Filmes | `/api/torrent-dos-filmes` |
| Vaca Torrent | `/api/vaca_torrent` |

For example, on the same host use `http://127.0.0.1:7006/api/rede_torrent`. When Prowlarr is on another device, bind the service to the LAN with `HOST=0.0.0.0` and use the server's LAN address. The API key field can be left empty.

Custom Cardigann definitions remain supported as an alternative. See [Prowlarr's custom definition instructions](https://wiki.servarr.com/prowlarr/indexers#adding-a-custom-yml-definition).

```yaml
---
id: torrent-indexer
name: Torrent Indexer
description: "Indexing Brazilian Torrent websites into structured data. github.com/felipemarinho97/torrent-indexer"
language: pt-BR
type: public
encoding: UTF-8
links:
  - http://localhost:8080/

caps:
  categories:
    Movies: Movies
    TV: TV

  modes:
    search: [q]
    tv-search: [q, season]
    movie-search: [q]

settings:
  - name: indexer
    type: select
    label: Indexer
    default: bludv
    options:
      search: Torrent-Indexer Cache
      bludv: BLUDV
      comando_torrents: Comando Torrents
      torrent-dos-filmes: Torrent dos Filmes
      starck-filmes: Starck Filmes
      rede_torrent: Rede Torrent
      vaca_torrent: Vaca Torrent

search:
  paths:
    - path: "{{ if eq .Config.indexer \"search\" }}/search{{ else }}/indexers/{{ .Config.indexer }}{{ end }}"
      response:
        type: json
  inputs:
    filter_results: "true"
    q: "{{ .Keywords }}"
  keywordsfilters:
    - name: tolower
    - name: re_replace
      args: ["(?i)(S0)(\\d{1,2})$", "temporada $2"]
    - name: re_replace
      args: ["(?i)(S)(\\d{1,3})$", "temporada $2"]

  rows:
    selector: $.results
    count:
      selector: $.count

  fields:
    download:
      selector: magnet_link
    title:
      selector: title
    description:
      selector: original_title
    details:
      selector: details
    infohash:
      selector: info_hash
    date:
      selector: date
    size:
      selector: size
    seeders:
      selector: seed_count
    leechers:
      selector: leech_count
    imdb:
      selector: imdb
    category_is_tv_show:
      selector: title
      filters:
        - name: regexp
          args: "\\b(S\\d+(?:E\\d+)?)\\b"
    category:
      text: "{{ if .Result.category_is_tv_show }}TV{{ else }}Movies{{ end }}"
```

## Integrated media tools

- [`integrations/homepage`](integrations/homepage/README.md): optional Homepage UI source for media requests, subtitles, status, and service shortcuts.
- [`integrations/glance`](integrations/glance/README.md): optional Glance dashboard widget for torrent-indexer availability.
- [`integrations/media-control/agent`](integrations/media-control/agent/README.md): optional local API backing that UI. It is separate from the torrent-indexer runtime.
- [`docs/FORK_CONSOLIDATION.md`](docs/FORK_CONSOLIDATION.md): source branches and integration notes.

## Web search dashboard

The project includes a torrent search interface at `/ui/`. Open `http://<server>:7006/ui/` when the service is reachable. Search uses Meilisearch, so set `MEILISEARCH_ADDRESS` (and `MEILISEARCH_KEY` when authentication is enabled) in the indexer's environment. Without a reachable Meilisearch instance, the page loads but search and index statistics report that the search service is unavailable. `/healthz` is a lightweight JSON liveness endpoint for dashboards such as Glance.

## Android / Termux

The ARM64 binary can run inside Debian under Termux without Docker. The phone-specific environment example and launch wrapper are in [`deploy/termux-debian`](deploy/termux-debian/README.md). FlareSolverr-Go works in sessionless mode; set `FLARESOLVERR_USE_SESSIONS=true` only when you need persistent browser sessions.


## Mirrors

- N/A

### Community Mirrors

If you're running a public instance of torrent-indexer and would like to share it with the community, please open a **Merge Request** adding your instance to this list!

**Requirements for community mirrors:**
- Must be publicly accessible
- Should have reasonable uptime
- Include geographic location information
- Provide contact information for maintenance issues

# Warning

The instance running at [https://torrent-indexer.darklyn.org/](https://torrent-indexer.darklyn.org/) is my personal instance and it is not guaranteed to be up all the time. Also, for better availability, I recommend deploying your own instance because the Cloudflare protection may block requests from indexed sites if too many requests are made in a short period of time from the same IP.

If I notice that the instance is being used a lot, I may block requests from Jackett/Prowlarr to avoid overloading the server. I can also redirect your requests to a fake instance without prior notice. Consider yourself warned.

## If you cannot self-host

**If you really need to use my [instance](https://torrent-indexer.darklyn.org) continuously without the risk of being IP blocked, please subscribe to my [GitHub Sponsors](https://github.com/sponsors/felipemarinho97) (any tier) and let me know about your support ❤️ via [Discord](https://discord.gg/7wqNywmpQW).**

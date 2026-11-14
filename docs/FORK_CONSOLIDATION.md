# Consolidated fork

This branch combines portable improvements from two community forks while keeping the project usable on ARM64 Android under Termux/Debian PRoot.

## Source branches

- [WagnerTB / `central_media_control`](https://github.com/WagnerTB/torrent-indexer/tree/central_media_control): native Torznab endpoints, domain reporting, season parsing, scraper improvements, a richer search UI, and optional Homepage/media-agent integrations.
- [guiga42 / `fix/scrapers-2026-10`](https://github.com/guiga42/torrent-indexer/tree/fix/scrapers-2026-10): updated scraper URLs, anti-bot detection, parser/decoder changes, and saved HTML fixtures with regression tests.

## Included

- Native Torznab endpoints at `/api/{indexer}` so Prowlarr can use each source as a Generic Torznab indexer.
- The `/domains` report, season/episode parsing, richer search UI, and Starck Filmes download-link resolution.
- Refreshed source URLs and scraper parsing from the 2026-10 branch, with environment overrides for future URL changes.
- FlareSolverr-Go support. Sessionless mode is the default; explicit session IDs and empty session-list responses are handled when session reuse is enabled.
- Host and metrics port configuration suitable for a local PRoot deployment. Phone deployment notes and sample settings are in `deploy/termux-debian`.
- Optional Homepage custom UI and media-agent source under `integrations/`.

## Kept out of the service core

The Raspberry Pi Docker Compose files, host addresses, storage paths, and hardware-specific deployment scripts from the media-control branch are not included. The Homepage integration detects the current hostname and supports explicit runtime configuration instead of embedding the contributor's LAN address. Keep credentials in local app configs or environment files; never commit them.

## Indexer URL overrides

The default domains are snapshots from the scraper branch. Override them without editing Go source:

- `INDEXER_COMANDO_URL`
- `INDEXER_BLUDV_URL`
- `INDEXER_TORRENT_DOS_FILMES_URL`
- `INDEXER_STARCK_FILMES_URL`
- `INDEXER_REDE_TORRENT_URL`
- `INDEXER_VACA_TORRENT_URL`

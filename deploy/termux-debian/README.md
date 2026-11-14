# Android: Termux + Debian PRoot

This deployment runs the Go service directly in Debian under Termux. Docker is not required. Redis and FlareSolverr-Go are separate local services.

## Build the ARM64 binary

From the repository root, on a machine with Go installed:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o torrent-indexer ./
```

Copy the binary into Debian as `/usr/local/bin/torrent-indexer`, then make it executable. Store the environment file at `/etc/torrent-indexer.env` with permissions `600`; start from [`torrent-indexer.env.example`](torrent-indexer.env.example).

## Run

Install [`run-torrent-indexer.sh`](run-torrent-indexer.sh) as `/usr/local/sbin/run-torrent-indexer`, then launch it through the existing Termux/PRoot supervisor:

```sh
proot-distro login --detach debian -- /usr/local/sbin/run-torrent-indexer
```

For this phone setup, Prowlarr and the indexer run in the same Debian environment, so the default `HOST=127.0.0.1` and the Torznab URL `http://127.0.0.1:7006/api/rede_torrent` work together. The metrics endpoint defaults to port `9081`, avoiding the phone's Nextcloud port `8081`.

The built-in search page is served at `http://<phone-lan-ip>:7006/ui/`. To open it from another device on the same LAN, set `HOST=0.0.0.0`; keep Prowlarr's Torznab URL on `127.0.0.1`. The search page requires a separate Meilisearch instance, for example `MEILISEARCH_ADDRESS=http://127.0.0.1:7700`. If Meilisearch is not configured, the page still loads but search is unavailable. The indexer's `/healthz` endpoint can be used by Glance; see [`integrations/glance`](../../integrations/glance/README.md).

## Meilisearch on ARM64

Download the official ARM64 binary, verify the release checksum, and install it:

```sh
curl -L https://github.com/meilisearch/meilisearch/releases/download/v1.56.2/meilisearch-linux-aarch64 -o /tmp/meilisearch
echo 'f379be855adaa102c7518228b5e1a33fb862f0f20acddc45720e74f0f54015a7  /tmp/meilisearch' | sha256sum -c -
install -m 0755 /tmp/meilisearch /usr/local/bin/meilisearch
install -m 0755 run-meilisearch.sh /usr/local/sbin/run-meilisearch
```

Start it through the Termux/PRoot supervisor with `proot-distro login --detach debian -- /usr/local/sbin/run-meilisearch`. The runner keeps Meilisearch on `127.0.0.1:7700` and stores its data under `/root/.local/share/meilisearch`; set `MEILISEARCH_ADDRESS=http://127.0.0.1:7700` for torrent-indexer. Do not bind Meilisearch to the LAN without setting an API key.

Set `FLARESOLVERR_ADDRESS=http://127.0.0.1:8191` and `FLARESOLVERR_USE_SESSIONS=false` for FlareSolverr-Go. Sessionless mode avoids API differences around session creation; persistent sessions are supported when enabled and require caller-generated IDs.

The server can bind to the LAN by setting `HOST=0.0.0.0`. Configure access controls in the network path before exposing the service beyond the phone.

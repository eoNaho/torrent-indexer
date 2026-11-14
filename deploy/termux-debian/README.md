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

Set `FLARESOLVERR_ADDRESS=http://127.0.0.1:8191` and `FLARESOLVERR_USE_SESSIONS=false` for FlareSolverr-Go. Sessionless mode avoids API differences around session creation; persistent sessions are supported when enabled and require caller-generated IDs.

The server can bind to the LAN by setting `HOST=0.0.0.0`. Configure access controls in the network path before exposing the service beyond the phone.

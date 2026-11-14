# Media-agent

Optional local API for the Homepage integration. It provides server status, storage, request management for Seerr/Radarr/Sonarr, and a Bazarr API proxy. The code has no embedded tokens or credentials; API keys are loaded from each app's configuration or from a configured key file.

Copy `env.example` to `/etc/media-agent.env`, adjust only the paths and URLs for apps you actually run, and restrict the file to the service account. Start with `LISTEN_HOST=127.0.0.1` and the loopback-only `ALLOWED_NETWORKS`. If a browser on another device must access the API, configure the listen address and allowed client subnet deliberately.

Run with:

```sh
./run.sh
```

The original Raspberry Pi deployment and hardware-specific voltage probe are not included. CPU, memory, uptime, storage, and app integrations are exposed independently of that hardware probe.

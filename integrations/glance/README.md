# Glance widget

This integration adds a compact status widget for the torrent-indexer HTTP
service and a link to its built-in search page. It uses Glance's built-in
`custom-api` widget and the indexer's `/healthz` endpoint.

1. Set `TORRENT_INDEXER_URL` in Glance's environment to the base URL reachable
   from the Glance process, without a trailing slash. For example,
   `http://127.0.0.1:7006` when both processes share the same network namespace,
   or `http://torrent-indexer:7006` when they share a Docker network.
2. Set `TORRENT_INDEXER_DASHBOARD_URL` to the browser-reachable URL of the
   search page, such as `http://192.168.0.114:7006/ui/` on a private LAN.
3. Include `torrent-indexer.yml` in a page's `widgets` list:

   ```yaml
   pages:
     - name: Servidor
       columns:
         - size: small
           widgets:
             - $include: /app/config/widgets/torrent-indexer.yml
   ```

4. Mount/copy this file to the path used by the `$include` and restart Glance
   (or wait for its config auto-reload).

The endpoint is a liveness check: it reports whether the indexer HTTP process
is responding. It does not check Redis, FlareSolverr, or individual torrent
sites. Keep the Glance URL on the private network unless you intentionally
publish the service.

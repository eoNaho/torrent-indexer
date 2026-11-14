# Glance widget

This integration adds an operational summary for the torrent-indexer: search
health, indexed release count, indexing state, and a link to the built-in
search page. It uses Glance's built-in `custom-api` widget and the indexer's
search endpoints.

1. Set `TORRENT_INDEXER_URL` in Glance's environment to the base URL reachable
   from the Glance process, without a trailing slash. For example,
   `http://127.0.0.1:7006` when both processes share the same network namespace,
   or `http://torrent-indexer:7006` when they share a Docker network.
2. Set `TORRENT_INDEXER_DASHBOARD_URL` to the browser-reachable URL of the
   search page, such as `http://192.168.0.114:7006/ui/` on a private LAN.
3. Append `torrent-indexer.css` to the CSS file already used by Glance. Keep
   your existing theme and widget styles.
4. Include `torrent-indexer.yml` in a page's `widgets` list:

   ```yaml
   pages:
     - name: Servidor
       columns:
         - size: small
           widgets:
             - $include: /app/config/widgets/torrent-indexer.yml
   ```

5. Mount/copy the widget YAML to the path used by `$include`, then restart Glance
   (or wait for its config auto-reload).

The widget reports the Meilisearch health and document count exposed by the
indexer. It does not check Redis, FlareSolverr, or individual torrent sites.
Keep the Glance URL on the private network unless you intentionally publish
the service.

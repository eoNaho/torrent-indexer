# Homepage media-control integration

Optional source modules for Homepage's custom JavaScript/CSS slot. This is a separate frontend integration; the torrent-indexer service does not require it.

The UI talks to a media-agent on port `8097` by default and can link to the media applications on the same host. The hostname is detected from the page URL. To override it, define `window.MEDIA_CONTROL_CONFIG` before the bundled script runs:

```js
window.MEDIA_CONTROL_CONFIG = {
  serviceHost: 'media-server.lan',
  mediaApiBaseUrl: 'http://media-server.lan:8097',
  gamingBaseUrl: 'http://media-server.lan:8099',
};
```

Build with Node.js 20+:

```sh
npm ci
npm run build
npm run check
```

Copy `dist/custom.js` and `custom.css` into your Homepage configuration and refresh the browser. The controls require the optional media-agent in [`../media-control/agent`](../media-control/agent/README.md). Configure the agent and its allowed client networks before enabling the controls.

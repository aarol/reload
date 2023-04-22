# Reload

![Tests](https://github.com/aarol/reload/actions/workflows/test.yml/badge.svg)

## Installation

```go get github.com/aarol/reload```

## Usage

1. Insert the WatchAndInject() middleware at the top of the request chain, specifying what directories to watch.
	```go
	var handler http.Handler = http.DefaultServeMux

	if isDevelopment {
		handler = reload.WatchAndInject("ui/")(handler)
	}

	http.ListenAndServe("localhost:3001", handler)
	```

2. (Optional) Use the reloader.OnReload callback to re-parse any cached templates
	```go
	reload.OnReload = func() {
		app.templateCache = newTemplateCache()
	}
	```
3. Run your application, make changes to any file in the specified directory, and see the updated page instantly!

See the full example at <https://github.com/aarol/reload/blob/main/example/main.go>

## Features

- Reloads the browser: the middleware injects a script into any html response sent by your server. The script will establish a Websocket connection to the server, which is then used to trigger a reload when any file under the specified directories changes.

## How it works

When added to the top of the middleware chain, it will inject a small \<script\> at the end of any HTML file sent by your application. This script will open a WebSocket connection to your server, also handled by the middleware.

## Caveats

* Reload works with everything that the server sends to the client (HTML,CSS,JS etc.), but it cannot reload the server itself, since it's just a middleware running on the server.

	To reload the entire server, you can use another file watcher on top, like [watchexec](https://github.com/watchexec/watchexec):

	```watchexec -r --exts .go -- go run .```

* Reload will not work for embedded assets, since all go:embed files are baked into the executable at build time.

If the built-in http.Handler middleware doesn't work for you,
you can still use the `ServeWS()`, `InjectScript()` and `Wait()` functions manually.

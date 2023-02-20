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

- Reloads the browser: The WatchAndReload middleware injects a script into any html response sent by your server. The script will establish a Websocket connection to the server, which is then used to trigger a reload when any file under the specified directories is changed.

- Displays a simple html error page on any >= 400 response, which will reload when any file is edited. This ensures that template errors (which sometimes cause a 500 internal server error), will not prevent the site from being reloaded.

## How it works

When added to the top of the middleware chain, it will write the raw response into a buffer. Depending on the body and Content-Type header, it will either

* Inject a small [\<script\>](https://github.com/aarol/reload/blob/2946b46da6a40f437029cf319b73c22ba550a924/reloader.go#L155) inside any HTML response that connects to a Websocket on the same host, also provided by the middleware.

* Convert the response into HTML and show a simple error page (also containing the [\<script\>](https://github.com/aarol/reload/blob/2946b46da6a40f437029cf319b73c22ba550a924/reloader.go#L155)) for any plaintext body with a statuscode >= 400

## Caveats

* Reload works with anything the server sends to the client (HTML,CSS,JS etc.), but cannot reload the server itself, since it's just a middleware running on the server.

	One workaround for this is to use another file watcher on top, like [watchexec](https://github.com/watchexec/watchexec):

	```watchexec -r --exts .go -- go run .```

* Reload will not work for embedded assets, since all go:embed files are baked into the executable at build time.

If the built-in http.Handler middleware doesn't work for you,
you can still use the `ServeWS()`, `InjectScript()` and `Wait()` functions manually.

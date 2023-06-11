# Reload

![Tests](https://github.com/aarol/reload/actions/workflows/test.yml/badge.svg)

Reload is a Go library, which enables "hot reloading" of web server assets and templates, updating the browser instantly via Websockets. The strength of Reload lies in it's simple API and easy integration to any Go projects.

## Installation

```go get github.com/aarol/reload```

## Usage

1. Insert the WatchAndInject() middleware at the top of the request chain, specifying what directories to watch.
	```go
	// this can be any http.Handler like chi.Router or gin.Engine
	var handler http.Handler = http.DefaultServeMux

	if isDevelopment {
		// specify which directories to watch recursively
		reload.Directories = []string{"ui/"}
		// inject the middleware
		// this will handle the Websocket connection and client side javascript
		handler = reload.Handle(handler)
	}

	http.ListenAndServe("localhost:3001", handler)
	```

2. (Optional) Use the reloader.OnReload callback to re-parse any cached templates
	```go
	reload.OnReload = func() {
		app.templateCache = newTemplateCache()
	}
	```
3. Run your application, make changes to files in the specified directory, and see the updated page instantly!

See the full example at <https://github.com/aarol/reload/blob/main/example/main.go>

## How it works

When added to the top of the middleware chain, `reload.Handle()` will inject a small \<script\> at the end of any HTML file sent by your application. This script will open a WebSocket connection to your server, also handled by the middleware.

> Currently, injecting the script is done by appending to the end of the document, even after the \</body\> tag. This makes the library code much simpler, but may break older/less forgiving browsers.

## Caveats

* Reload works with everything that the server sends to the client (HTML,CSS,JS etc.), but it cannot restart the server itself, since it's just a middleware running on the server.

	To reload the entire server, you can use another file watcher on top, like [watchexec](https://github.com/watchexec/watchexec):

	```watchexec -r --exts .go -- go run .```

* Reload will not work for embedded assets, since all go:embed files are baked into the executable at build time. 

If the built-in http.Handler middleware doesn't work for you,
you can still use the `ServeWS()`, `InjectScript()` and `Wait()` functions manually.

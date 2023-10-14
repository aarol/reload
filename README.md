# Reload

![Tests](https://github.com/aarol/reload/actions/workflows/test.yml/badge.svg)

Reload is a Go library, which enables "hot reloading" of web server assets and templates, reloading the browser instantly via Websockets. The strength of Reload lies in it's simple API and easy integration to any Go projects.

## Installation

`go get github.com/aarol/reload`

## Usage

1. Insert the Handle() middleware at the top of the request chain and set the directories that should be watched

   ```go
   var handler http.Handler = http.DefaultServeMux
   // or chi.Router or gin.Engine or echo.Echo

   if isDevelopment {
    	reload.Directories = []string{"ui/"}
    	handler = reload.Handle(handler)
   }

   http.ListenAndServe(addr, handler)
   ```

2. (Optional) Use the `reload.OnReload` callback to re-parse any templates

   ```go
   reload.OnReload = func() {
   	app.parseTemplates()
   }
   ```

3. Run your application, make changes to files in the specified directory, and see the updated page instantly!

See the full example at <https://github.com/aarol/reload/blob/main/example/main.go>

## How it works

When added to the top of the middleware chain, `reload.Handle()` will inject a small \<script\> at the end of any HTML file sent by your application. This script will instruct the browser to open a WebSocket connection back to your server, which will be also handled by the middleware.

The injected script is at the bottom of [this file](https://github.com/aarol/reload/blob/main/reloader.go). If you want to do the injection yourself, you can just copy the script from the source and set `reload.DisableInject` to `true`.

The package also exposes `ServeWS`, `InjectScript`, `Wait` and `WatchDirectories`, which can be used instead of the `Handle` middleware.

> Currently, injecting the script is done by appending to the end of the document, even after the \</html\> tag. This makes the library code *much* simpler, but may break older/less forgiving browsers.

## Caveats

- Reload works with everything that the server sends to the client (HTML,CSS,JS etc.), but it cannot restart the server itself, since it's just a middleware running on the server.

  To reload the entire server, you can use another file watcher on top, like [watchexec](https://github.com/watchexec/watchexec):

  `watchexec -r --exts .go -- go run .`

- Reload will not work for embedded assets, since all go:embed files are baked into the executable at build time.

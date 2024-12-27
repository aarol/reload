# Reload

Reload is a Go package, which enables "soft reloading" of web server assets and templates, reloading the browser instantly via Websockets. The strength of Reload lies in it's simple API and easy integration to any Go projects.

## Installation

`go get github.com/aarol/reload`

## Usage

1. Create a new Reloader and insert the middleware to your handler chain:

   ```go
   // handler can be anything that implements http.Handler,
   // like chi.Router, echo.Echo or gin.Engine
   var handler http.Handler = http.DefaultServeMux

   if isDevelopment {
      // Call `New()` with a list of directories to recursively watch
      reloader := reload.New("ui/")

      // Optionally, define a callback to
      // invalidate any caches
      reloader.OnReload = func() {
         app.parseTemplates()
      }

      // Use the Handle() method as a middleware
      handler = reloader.Handle(handler)
   }

   http.ListenAndServe(addr, handler)
   ```

2. Run your application, make changes to files in the specified directories, and see the updated page instantly!

See the full example at <https://github.com/aarol/reload/blob/main/example/main.go>

> [!NOTE]  
> The browser often caches resources served by http.FileServer (because of Last-Modified headers generated from filesystem timestamps),
> which leads to unexpected caching issues when using this middleware. To prevent this, the middleware sends a "Cache-Control: no-cache" header. This is enabled by default, but can be disabled by setting the `DisableCaching` field to `false`.

### Logging

By default, reload logs errors on `stderr`, with `(*Reloader).ErrorLog`. Error logging can be disabled by setting `ErrorLog` to `nil`.

There is also a logger which reports debug information, exposed as `(*Reloader).DebugLog` (`nil` by default)

## How it works

When added to the top of the middleware chain, `(*Reloader).Handle()` will inject a small `<script/>` at the end of any HTML file sent by your application. This script will instruct the browser to open a WebSocket connection back to your server, which will be also handled by the middleware.

The injected script is at the bottom of [this file](https://github.com/aarol/reload/blob/main/reload.go).

You can also do the injection yourself, as the package also exposes the methods `(*Reloader).ServeWS` and `(*Reloader).WatchDirectories`, which are used by the `(*Reloader).Handle` middleware.

> Currently, injecting the script is done by appending to the end of the document, even after the \</html\> tag.
> This makes the library code _much_ simpler, but may break older/less forgiving browsers.

## Caveats

- Reload works with everything that the server sends to the client (HTML,CSS,JS etc.), but it cannot restart the server itself,
  since it's just a middleware running on the server.

  To reload the entire server, you can use another file watcher on top, like [watchexec](https://github.com/watchexec/watchexec):

  `watchexec -r --exts .go -- go run .`

  When the websocket connection to the server is lost, the browser will try to reconnect every second.
  This means that when the server comes back, the browser will still reload, although not as fast :)

- Reload will not work for embedded assets, since all go:embed files are baked into the executable at build time.

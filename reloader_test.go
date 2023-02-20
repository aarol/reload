package reload

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

// Can reach websocket handled by middleware
func TestWebsocket(t *testing.T) {
	ts := httptest.NewServer(WatchAndInject()(http.NotFoundHandler()))

	defer ts.Close()

	url, _ := url.Parse(ts.URL)
	url.Scheme = "ws"
	url.Path = "/reload"
	conn, res, err := websocket.DefaultDialer.Dial(url.String(), nil)
	assert.Nil(t, err)

	assert.Equal(t, res.StatusCode, http.StatusSwitchingProtocols)

	// trigger cond.Wait() in ServeWS()
	cond.Broadcast()
	_, msg, err := conn.ReadMessage()

	assert.Nil(t, err)

	assert.Equal(t, string(msg), "reload")
}

// Middleware only converts errors into html
func TestContentType(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte("body {}"))
	})

	mux.HandleFunc("/js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Write([]byte("1+2"))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<!DOCTYPE html> "))
	})

	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", 500)
	})

	mux.HandleFunc("/notfound", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	mux.HandleFunc("/plain", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("plain text"))
	})

	ts := httptest.NewServer(WatchAndInject()(mux))

	testdata := []struct {
		path        string
		contentType string
		statusCode  int
	}{
		{
			"/css",
			"text/css; charset=utf-8",
			200,
		},
		{
			"/js",
			"text/javascript; charset=utf-8",
			200,
		},
		{
			"/",
			"text/html; charset=utf-8",
			200,
		},
		{
			"/notfound",
			"text/html; charset=utf-8",
			404,
		},
		{
			"/error",
			"text/html; charset=utf-8",
			500,
		},
		{
			"/plain",
			"text/plain; charset=utf-8",
			200,
		},
	}

	for _, tt := range testdata {

		res, err := http.Get(ts.URL + tt.path)
		assert.Nil(t, err)
		assert.Equal(t, res.StatusCode, tt.statusCode)
		assert.Equal(t, res.Header.Get("Content-Type"), tt.contentType)
	}
}

func TestInject(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body := `<!DOCTYPE html>
		<html lang="en">
		
		<head>
			<title>Document</title>
			<link rel="stylesheet" href="/static/index.css">
		</head>
		
		<body>
			<h1>Hello World!</h1>
		</body>`
		w.Write([]byte(body))
	})

	ts := httptest.NewServer(WatchAndInject()(mux))

	res, err := http.Get(ts.URL)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	assert.Nil(t, err)
	bodyIndex := strings.Index(string(body), "<body>")
	closingBodyIndex := strings.Index(string(body), "</body>")
	scriptIndex := strings.Index(string(body), "<script>")
	assert.Greater(t, scriptIndex, bodyIndex)
	assert.Greater(t, closingBodyIndex, scriptIndex)
}

// Partial html response, response with modified body tag
func TestPartialResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body := `<!DOCTYPE html>
		<html lang="en">
		
		<head>
			<title>Document</title>
			<link rel="stylesheet" href="/static/index.css">
		</head>
		
		<body>
			<h1>Hello World!</h1>`
		w.Write([]byte(body))
	})
	mux.HandleFunc("/modified", func(w http.ResponseWriter, r *http.Request) {
		body := `<!DOCTYPE html>
		<html lang="en">
		
		<head>
			<title>Document</title>
			<link rel="stylesheet" href="/static/index.css">
		</head>
		
		<body class="main" asdf>
			<h1>Hello World!</h1>`
		w.Write([]byte(body))
	})

	ts := httptest.NewServer(WatchAndInject()(mux))

	for _, path := range []string{"/", "/modified"} {
		res, err := http.Get(ts.URL + path)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		assert.Nil(t, err)
		bodyIndex := strings.Index(string(body), "<body")
		scriptIndex := strings.Index(string(body), "<script>")
		assert.Greater(t, scriptIndex, bodyIndex)
	}
}

var benchBody = `<!DOCTYPE html>
<html lang="en">

<head>
	<title>Document</title>
	<link rel="stylesheet" href="/static/index.css">
</head>

<body>
	<h1>Hello World!</h1>
</body>`

func Benchmark(b *testing.B) {
	Logger.SetOutput(io.Discard)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(benchBody))
	})
	ts := httptest.NewServer(WatchAndInject()(mux))
	b.Run("middlware", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			res, _ := http.Get(ts.URL)
			if res.StatusCode != 200 {
				b.Error(res.StatusCode)
			}
		}
	})
	ts.Close()
	ts = httptest.NewServer(mux)
	b.Run("default", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			res, _ := http.Get(ts.URL)
			if res.StatusCode != 200 {
				b.Error(res.StatusCode)
			}
		}
	})
	ts.Close()
}

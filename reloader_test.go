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
	ts := httptest.NewServer(Handle(http.NotFoundHandler()))

	defer ts.Close()

	url, _ := url.Parse(ts.URL)
	url.Scheme = "ws"
	url.Path = Endpoint
	conn, res, err := websocket.DefaultDialer.Dial(url.String(), nil)
	assert.NoError(t, err)

	assert.Equal(t, res.StatusCode, http.StatusSwitchingProtocols)

	// trigger cond.Wait() in ServeWS()
	cond.Broadcast()
	_, msg, err := conn.ReadMessage()

	assert.NoError(t, err)

	assert.Equal(t, string(msg), "reload")
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

	ts := httptest.NewServer(Handle(mux))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(body), "script")
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

	ts := httptest.NewServer(Handle(mux))
	defer ts.Close()

	for _, path := range []string{"/", "/modified"} {
		res, err := http.Get(ts.URL + path)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		assert.NoError(t, err)
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
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(benchBody))
	})

	ts := httptest.NewServer(mux)
	b.Run("default", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			res, _ := http.Get(ts.URL)
			if res.StatusCode != 200 {
				b.Error(res.StatusCode)
			}
		}
	})
	ts.Close()
	Log.SetOutput(io.Discard)
	ts = httptest.NewServer(Handle(mux))
	b.Run("middlware", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			res, _ := http.Get(ts.URL)
			if res.StatusCode != 200 {
				b.Error(res.StatusCode)
			}
		}
	})
	ts.Close()
}

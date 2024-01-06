package reload

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func NewTestingReloader(t *testing.T) *Reloader {
	t.Helper()

	r := New(t.TempDir())

	r.Log.SetOutput(io.Discard)

	return r
}

func TestReload(t *testing.T) {
	indexHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!DOCTYPE html>
		<html lang="en">

		<head>
			<title>Document</title>
			<link rel="stylesheet" href="/static/index.css">
		</head>

		<body>
			<h1>Hello World!</h1>
		</body>`)
	})

	reload := NewTestingReloader(t)

	ts := httptest.NewServer(reload.Handle(indexHandler))
	defer ts.Close()

	conn, res := newWebsocketConn(t, ts.URL+reload.Endpoint, wsCurrentVersion)
	assert.Equal(t, http.StatusSwitchingProtocols, res.StatusCode)

	res, err := http.Get(ts.URL)
	assert.NoError(t, err)
	defer res.Body.Close()

	assert.True(t, containsScript(t, reload, res.Body))

	// create file in watched directory
	f, err := os.Create(filepath.Join(reload.directories[0], "temp.txt"))
	assert.NoError(t, err)
	defer f.Close()

	msgType, data, err := conn.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, msgType)
	assert.Equal(t, "reload", string(data))
}

func TestWebsocketOldVersion(t *testing.T) {
	reload := NewTestingReloader(t)

	ts := httptest.NewServer(reload.Handle(http.NotFoundHandler()))

	defer ts.Close()
	b := bytes.Buffer{}
	reload.Log.SetOutput(&b)
	_, res := newWebsocketConn(t, ts.URL+reload.Endpoint, "0")
	assert.Equal(t, http.StatusSwitchingProtocols, res.StatusCode)

	assert.Contains(t, b.String(), "out of date")
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

	reload := NewTestingReloader(t)

	ts := httptest.NewServer(reload.Handle(mux))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	defer res.Body.Close()

	assert.True(t, containsScript(t, reload, res.Body))
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

	reload := NewTestingReloader(t)

	ts := httptest.NewServer(reload.Handle(mux))
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

	reload := New(b.TempDir())

	reload.Log.SetOutput(io.Discard)

	ts = httptest.NewServer(reload.Handle(mux))
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

func containsScript(t *testing.T, reload *Reloader, responseBody io.Reader) bool {
	body, err := io.ReadAll(responseBody)
	assert.NoError(t, err)
	return strings.Contains(string(body), InjectedScript(reload.Endpoint))
}

func newWebsocketConn(t *testing.T, addr string, wsVersion string) (*websocket.Conn, *http.Response) {
	t.Helper()

	url, err := url.Parse(addr)
	assert.NoError(t, err)
	url.Scheme = "ws"
	q := url.Query()
	q.Add("v", wsVersion)
	url.RawQuery = q.Encode()

	conn, res, err := websocket.DefaultDialer.Dial(url.String(), nil)
	assert.NoError(t, err)
	return conn, res
}

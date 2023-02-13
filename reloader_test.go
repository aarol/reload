package reload

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

// Can reach websocket handled by middleware
func TestWebsocket(t *testing.T) {
	ts := httptest.NewServer(Inject(http.DefaultServeMux))

	defer ts.Close()

	url, _ := url.Parse(ts.URL)
	url.Scheme = "ws"
	url.Path = "/reload"
	conn, res, err := websocket.DefaultDialer.Dial(url.String(), nil)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, res.StatusCode, 101)

	cond.Broadcast()
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, string(msg), "reload")
}

// Middleware only converts errors into html
func TestContentType(t *testing.T) {
	http.HandleFunc("/css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write([]byte("body {}"))
	})

	http.HandleFunc("/js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Write([]byte("1+2"))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<!DOCTYPE html> "))
	})

	http.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", 500)
	})

	http.HandleFunc("/notfound", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	http.HandleFunc("/plain", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("plain text"))
	})

	ts := httptest.NewServer(Inject(http.DefaultServeMux))

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
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, res.StatusCode, tt.statusCode)
		assert.Equal(t, res.Header.Get("Content-Type"), tt.contentType)
	}
}

// TODO: partial html response, response with modified body tag

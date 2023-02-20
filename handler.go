package reload

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
)

type wrapper struct {
	http.ResponseWriter
	header int
	buf    *bytes.Buffer
}

func (w *wrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		Logger.Println("HTTP handler called Hijack() but the underlying responseWriter did not support it")
	}
	return hijacker.Hijack()
}

func (w *wrapper) Flush() {
	flusher, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		Logger.Println("HTTP handler called Flush() but the underlying responseWriter did not support it")
	}
	flusher.Flush()
}

func (w *wrapper) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		Logger.Println("HTTP handler called Push() but the underlying responseWriter did not support it")
	}
	return pusher.Push(target, opts)
}

func (w *wrapper) ReadFrom(r io.Reader) (n int64, err error) {
	return w.buf.ReadFrom(r)
}

func (w *wrapper) WriteHeader(code int) {
	w.header = code
}

func (w *wrapper) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func insertScriptIntoHTML(src, script []byte) []byte {
	buf := &bytes.Buffer{}

	index := bytes.Index(src, []byte("</body>"))
	if index != -1 {
		// insert before closing body tag
		buf.Write(src[:index])
		buf.Write(script)
		buf.Write(src[index:])

		return buf.Bytes()
	} else {
		// insert after beginning body tag
		match := []byte("<body")
		index = bytes.Index(src, match)
		if index != -1 {
			// find end of body tag
			offset := bytes.IndexRune(src[index:], '>')

			if offset != -1 {
				buf.Write(src[:index+len(match)+offset])
				buf.Write(script)
				buf.Write(src[index+len(match)+offset:])

				return buf.Bytes()
			}
		}
	}

	return src
}

package reload

import (
	"bufio"
	"bytes"
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

func (w *wrapper) WriteHeader(code int) {
	w.header = code
}

func findAndInsertBefore(src, match []byte, value string) []byte {
	index := bytes.Index(src, match)
	buf := &bytes.Buffer{}
	if index == -1 {
		buf.Write(src)
	} else {
		buf.Write(src[:index])
		buf.WriteString(value)
		buf.Write(src[index:])
	}
	return buf.Bytes()
}

func (w *wrapper) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

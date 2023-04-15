package reload

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
)

func newWrapResponseWriter(w http.ResponseWriter, protoMajor int) WrapResponseWriter {
	_, fl := w.(http.Flusher)

	bw := basicWriter{ResponseWriter: w, body: &bytes.Buffer{}}

	if protoMajor == 2 {
		_, ps := w.(http.Pusher)
		if fl && ps {
			return &http2FancyWriter{bw}
		}
	} else {
		_, hj := w.(http.Hijacker)
		_, rf := w.(io.ReaderFrom)
		if fl && hj && rf {
			return &httpFancyWriter{bw}
		}
		if fl && hj {
			return &flushHijackWriter{bw}
		}
		if hj {
			return &hijackWriter{bw}
		}
	}

	if fl {
		return &flushWriter{bw}
	}

	return &bw
}

// WrapResponseWriter is copied from chi's middleware (https://github.com/go-chi/chi/blob/master/middleware/wrap_writer.go) under MIT license
// It keeps the response body in a buffer
type WrapResponseWriter interface {
	http.ResponseWriter
	// Status returns the HTTP status of the request, or 0 if one has not
	// yet been sent.
	Status() int
	// Unwrap returns the original proxied target.
	Unwrap() http.ResponseWriter
	Body() []byte
}

// basicWriter wraps a http.ResponseWriter that implements the minimal
// http.ResponseWriter interface.
type basicWriter struct {
	http.ResponseWriter
	wroteHeader bool
	code        int
	body        *bytes.Buffer
}

func (b *basicWriter) WriteHeader(code int) {
	if !b.wroteHeader {
		b.code = code
		b.wroteHeader = true
	}
}

func (b *basicWriter) Write(buf []byte) (int, error) {
	b.maybeWriteHeader()
	return b.body.Write(buf)
}

func (b *basicWriter) maybeWriteHeader() {
	if !b.wroteHeader {
		b.WriteHeader(http.StatusOK)
	}
}

func (b *basicWriter) Status() int {
	return b.code
}

func (b *basicWriter) Unwrap() http.ResponseWriter {
	return b.ResponseWriter
}

func (b *basicWriter) Body() []byte {
	return b.body.Bytes()
}

// flushWriter ...
type flushWriter struct {
	basicWriter
}

// Body implements WrapResponseWriter
func (f *flushWriter) Body() []byte {
	return f.body.Bytes()
}

func (f *flushWriter) Flush() {
	f.wroteHeader = true
	fl := f.basicWriter.ResponseWriter.(http.Flusher)
	fl.Flush()
}

var _ http.Flusher = &flushWriter{}

// hijackWriter ...
type hijackWriter struct {
	basicWriter
}

// Body implements WrapResponseWriter
func (f *hijackWriter) Body() []byte {
	return f.body.Bytes()
}

func (f *hijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj := f.basicWriter.ResponseWriter.(http.Hijacker)
	return hj.Hijack()
}

var _ http.Hijacker = &hijackWriter{}

// flushHijackWriter ...
type flushHijackWriter struct {
	basicWriter
}

// Body implements WrapResponseWriter
func (f *flushHijackWriter) Body() []byte {
	return f.body.Bytes()
}

func (f *flushHijackWriter) Flush() {
	f.wroteHeader = true
	fl := f.basicWriter.ResponseWriter.(http.Flusher)
	fl.Flush()
}

func (f *flushHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj := f.basicWriter.ResponseWriter.(http.Hijacker)
	return hj.Hijack()
}

var (
	_ http.Flusher  = &flushHijackWriter{}
	_ http.Hijacker = &flushHijackWriter{}
)

// httpFancyWriter is a HTTP writer that additionally satisfies
// http.Flusher, http.Hijacker, and io.ReaderFrom. It exists for the common case
// of wrapping the http.ResponseWriter that package http gives you, in order to
// make the proxied object support the full method set of the proxied object.
type httpFancyWriter struct {
	basicWriter
}

// Body implements WrapResponseWriter
func (f *httpFancyWriter) Body() []byte {
	return f.body.Bytes()
}

func (f *httpFancyWriter) Flush() {
	f.wroteHeader = true
	fl := f.basicWriter.ResponseWriter.(http.Flusher)
	fl.Flush()
}

func (f *httpFancyWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj := f.basicWriter.ResponseWriter.(http.Hijacker)
	return hj.Hijack()
}

func (f *http2FancyWriter) Push(target string, opts *http.PushOptions) error {
	return f.basicWriter.ResponseWriter.(http.Pusher).Push(target, opts)
}

func (f *httpFancyWriter) ReadFrom(r io.Reader) (int64, error) {
	return f.body.ReadFrom(r)
}

var (
	_ http.Flusher  = &httpFancyWriter{}
	_ http.Hijacker = &httpFancyWriter{}
	_ http.Pusher   = &http2FancyWriter{}
	_ io.ReaderFrom = &httpFancyWriter{}
)

// http2FancyWriter is a HTTP2 writer that additionally satisfies
// http.Flusher, and io.ReaderFrom. It exists for the common case
// of wrapping the http.ResponseWriter that package http gives you, in order to
// make the proxied object support the full method set of the proxied object.
type http2FancyWriter struct {
	basicWriter
}

func (f *http2FancyWriter) Body() []byte {
	return f.body.Bytes()
}

func (f *http2FancyWriter) Flush() {
	f.wroteHeader = true
	fl := f.basicWriter.ResponseWriter.(http.Flusher)
	fl.Flush()
}

var _ http.Flusher = &http2FancyWriter{}

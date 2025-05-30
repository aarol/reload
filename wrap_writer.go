package reload

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// The following was copied and modified from Chi's WrapWriter middleware (https://github.com/go-chi/chi/blob/6ceb498b221efa11f559602beb2463b8b52c2161/middleware/wrap_writer.go)
// The original work was derived from Goji's middleware (https://github.com/zenazn/goji/tree/master/web/middleware)

func newWrapResponseWriter(w http.ResponseWriter, protoMajor int, scriptSize int) wrapResponseWriter {
	_, fl := w.(http.Flusher)

	bw := basicWriter{ResponseWriter: w, scriptSize: scriptSize}

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

// wrapResponseWriter is a proxy around an http.ResponseWriter that allows you to hook
// into various parts of the response process.
type wrapResponseWriter interface {
	http.ResponseWriter
	// Status returns the HTTP status of the request, or 0 if one has not
	// yet been sent.
	Status() int
	// Tee causes the response body to be written to the given io.Writer in
	// addition to proxying the writes through. Only one io.Writer can be
	// tee'd to at once: setting a second one will overwrite the first.
	// Writes will be sent to the proxy before being written to this
	// io.Writer. It is illegal for the tee'd writer to be modified
	// concurrently with writes.
	Tee(io.Writer)
	// Unwrap returns the original proxied target.
	Unwrap() http.ResponseWriter
}

// basicWriter wraps a http.ResponseWriter that implements the minimal
// http.ResponseWriter interface.
type basicWriter struct {
	http.ResponseWriter
	scriptSize  int
	wroteHeader bool
	code        int
	bytes       int
	tee         io.Writer
}

func (b *basicWriter) WriteHeader(code int) {

	headers := b.Header()

	// If the Content-Length is set, and the Content-Type is HTML, then injecting
	// the script at the end will not work (typically this is done by http.FileServer)
	//
	// Handle this by changing Content-Size before sending the headers to include space for the script
	if contentLength := headers.Get("Content-Length"); contentLength != "" {
		if strings.HasPrefix(headers.Get("Content-Type"), "text/html") {
			cl, err := strconv.Atoi(contentLength)
			// ignore if failed to parse
			if err == nil {
				newLength := strconv.Itoa(cl + b.scriptSize)
				headers.Set("Content-Length", newLength)
			}
		}
	}

	if !b.wroteHeader {
		b.code = code
		b.wroteHeader = true
		b.ResponseWriter.WriteHeader(code)
	}
}

func (b *basicWriter) Write(buf []byte) (int, error) {
	b.maybeWriteHeader()
	n, err := b.ResponseWriter.Write(buf)
	if b.tee != nil {
		b.tee.Write(buf[:n])
	}
	b.bytes += n
	return n, err
}

func (b *basicWriter) maybeWriteHeader() {
	if !b.wroteHeader {
		b.WriteHeader(http.StatusOK)
	}
}

func (b *basicWriter) Status() int {
	return b.code
}

func (b *basicWriter) BytesWritten() int {
	return b.bytes
}

func (b *basicWriter) Tee(w io.Writer) {
	b.tee = w
}

func (b *basicWriter) Unwrap() http.ResponseWriter {
	return b.ResponseWriter
}

// flushWriter ...
type flushWriter struct {
	basicWriter
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

func (f *hijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj := f.basicWriter.ResponseWriter.(http.Hijacker)
	return hj.Hijack()
}

var _ http.Hijacker = &hijackWriter{}

// flushHijackWriter ...
type flushHijackWriter struct {
	basicWriter
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
	if f.basicWriter.tee != nil {
		n, err := io.Copy(&f.basicWriter, r)
		f.basicWriter.bytes += int(n)
		return n, err
	}
	rf := f.basicWriter.ResponseWriter.(io.ReaderFrom)
	f.basicWriter.maybeWriteHeader()
	n, err := rf.ReadFrom(r)
	f.basicWriter.bytes += int(n)
	return n, err
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

func (f *http2FancyWriter) Flush() {
	f.wroteHeader = true
	fl := f.basicWriter.ResponseWriter.(http.Flusher)
	fl.Flush()
}

var _ http.Flusher = &http2FancyWriter{}

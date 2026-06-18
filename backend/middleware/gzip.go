package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

var gzipPool = sync.Pool{
	New: func() interface{} {
		gz, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return gz
	},
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
	wroteHeader bool
}

func (grw *gzipResponseWriter) WriteHeader(code int) {
	if !grw.wroteHeader {
		grw.wroteHeader = true
		if code >= 300 && code < 400 {
			grw.ResponseWriter.Header().Del("Content-Encoding")
		} else if grw.Header().Get("Content-Type") != "" {
			grw.ResponseWriter.Header().Set("Content-Encoding", "gzip")
			grw.ResponseWriter.Header().Del("Content-Length")
		}
	}
	grw.ResponseWriter.WriteHeader(code)
}

func (grw *gzipResponseWriter) Write(b []byte) (int, error) {
	if !grw.wroteHeader {
		grw.WriteHeader(http.StatusOK)
	}
	return grw.Writer.Write(b)
}

func (grw *gzipResponseWriter) Flush() {
	if f, ok := grw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func GzipCompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ae := r.Header.Get("Accept-Encoding")
		if !strings.Contains(ae, "gzip") || r.Header.Get("Upgrade") != "" {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path
		compressible := false
		switch {
		case strings.HasSuffix(path, ".js"):
			compressible = true
		case strings.HasSuffix(path, ".css"):
			compressible = true
		case strings.HasSuffix(path, ".html"):
			compressible = true
		case strings.HasSuffix(path, ".json"):
			compressible = true
		case strings.HasSuffix(path, ".svg"):
			compressible = true
		case strings.HasSuffix(path, ".xml"):
			compressible = true
		case strings.HasPrefix(path, "/api/"):
			compressible = true
		case strings.HasPrefix(path, "/metrics"):
			compressible = true
		}
		if !compressible {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzipPool.Get().(*gzip.Writer)
		defer func() {
			gz.Close()
			gzipPool.Put(gz)
		}()

		w.Header().Set("Vary", "Accept-Encoding")

		grw := &gzipResponseWriter{
			Writer:         gz,
			ResponseWriter: w,
		}
		gz.Reset(grw.ResponseWriter)
		defer gz.Reset(io.Discard)

		next.ServeHTTP(grw, r)
	})
}

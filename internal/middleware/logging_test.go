package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggingStatusCode(t *testing.T) {
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/img.png", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestLoggingBytesWritten(t *testing.T) {
	payload := []byte("hello world")
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))

	req := httptest.NewRequest(http.MethodGet, "/img.png", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Body.Bytes(); string(got) != string(payload) {
		t.Errorf("body = %q, want %q", got, payload)
	}
}

func TestLoggingDefaultStatus200(t *testing.T) {
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestResponseWriterWriteAccumulates(t *testing.T) {
	base := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: base, statusCode: http.StatusOK}

	n1, _ := rw.Write([]byte("abc"))
	n2, _ := rw.Write([]byte("de"))

	if rw.bytesWritten != n1+n2 {
		t.Errorf("bytesWritten = %d, want %d", rw.bytesWritten, n1+n2)
	}
	if rw.bytesWritten != 5 {
		t.Errorf("bytesWritten = %d, want 5", rw.bytesWritten)
	}
}

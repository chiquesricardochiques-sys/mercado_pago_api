package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"ricardo-payments/internal/apperr"
	"ricardo-payments/internal/modules/pay"
	"ricardo-payments/internal/modules/receive"
)

type Deps struct {
	APIKey  string
	Receive *receive.Service
	Pay     *pay.Service
}

func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})

	mux.Handle("POST /receive", auth(d.APIKey, postJSON(d.Receive.Create)))
	mux.Handle("GET /receive/{id}", auth(d.APIKey, getByID(d.Receive.Get)))
	mux.Handle("POST /pay", auth(d.APIKey, postJSON(d.Pay.Pay)))
	mux.Handle("GET /pay/{id}", auth(d.APIKey, getByID(d.Pay.Get)))

	return recoverer(requestLog(withTimeout(mux, 25*time.Second)))
}

// postJSON lê o JSON (recusando campos desconhecidos e corpos grandes) e chama o serviço.
func postJSON[In, Out any](fn func(context.Context, In) (Out, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in In
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			writeError(w, apperr.Validation("JSON inválido: "+err.Error()))
			return
		}
		out, err := fn(r.Context(), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 200, out)
	}
}

func getByID[Out any](fn func(context.Context, string) (Out, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := fn(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 200, out)
	}
}

func auth(key string, next http.Handler) http.Handler {
	want := sha256.Sum256([]byte(key))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := sha256.Sum256([]byte(r.Header.Get("X-API-Key")))
		if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			writeJSON(w, 401, &apperr.Error{Code: "unauthorized", Message: "não autorizado"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withTimeout(next http.Handler, d time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// requestLog registra método, caminho, status e duração (nunca o corpo nem os headers).
func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newID()
		w.Header().Set("X-Request-Id", id)
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		start := time.Now()
		next.ServeHTTP(rec, r)
		log.Printf("req=%s %s %s -> %d (%s)", id, r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC %s %s: %v", r.Method, r.URL.Path, rec)
				writeJSON(w, 500, &apperr.Error{Code: "internal_error", Message: "erro interno"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeError(w http.ResponseWriter, err error) {
	if ae, ok := apperr.As(err); ok {
		writeJSON(w, ae.Status, ae)
		return
	}
	log.Printf("erro interno: %v", err)
	writeJSON(w, 500, &apperr.Error{Code: "internal_error", Message: "erro interno"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
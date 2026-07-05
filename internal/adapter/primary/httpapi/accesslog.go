package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// accessLog — паритет telegram Log-middleware: метод, путь, статус,
// длительность каждого запроса. Одно отличие: успешный /health уходит на
// Debug — панель поллит его постоянно, на Info он вымыл бы 64КБ-кольцо
// logread (тот же аргумент, что state-transition логи поллера).
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		level := slog.LevelInfo
		if r.URL.Path == healthPath && sw.status < http.StatusBadRequest {
			level = slog.LevelDebug
		}
		s.logger.Log(r.Context(), level, "http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration", time.Since(start).Round(time.Millisecond),
		)
	})
}

// statusWriter перехватывает код ответа для access-лога. Flush/Hijack
// сознательно не пробрасываются: стриминга и websocket в API нет.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const demoUser = "demo"

var demoHash = []byte("$2a$10$tkrkFkmonxehpcmEJD6M4OAhT8gDEG4lz6VxaaFdZnVP4bc87rpIa")

func login(secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid login")
			return
		}
		if body.Username != demoUser || bcrypt.CompareHashAndPassword(demoHash, []byte(body.Password)) != nil {
			writeError(w, http.StatusUnauthorized, "invalid login")
			return
		}
		raw, err := sign(secret, demoUser)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "login")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"token": raw})
	}
}

func sign(secret, subject string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   subject,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func guard(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if r.URL.Path == "/health" || r.URL.Path == "/login" {
			next.ServeHTTP(w, r)
			return
		}
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || strings.TrimSpace(raw) == "" || secret == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

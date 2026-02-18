package server

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) clientPasswordAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pw := r.Header.Get("X-Client-Password")
		if pw == "" {
			http.Error(w, `{"error":"missing X-Client-Password header"}`, http.StatusUnauthorized)
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(s.cfg.ClientPasswordHash), []byte(pw)); err != nil {
			http.Error(w, `{"error":"invalid password"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) adminBasicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(s.cfg.AdminPasswordHash), []byte(pass)); err != nil {
			http.Error(w, `{"error":"invalid password"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

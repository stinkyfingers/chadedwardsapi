package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var ErrTokenExpired = errors.New("token is expired")

type GCP struct {
	TokenInfo TokenInfo
}

type TokenInfo struct {
	Email         string `json:"email"`
	IssuedTo      string `json:"issued_to"`
	Audience      string `json:"audience"`
	UserId        string `json:"user_id"`
	ExpiresIn     int    `json:"expires_in"`
	EmailVerified bool   `json:"email_verified"`
	AccessType    string `json:"access_type"`
	Scope         string `json:"scope"`
}

func (g *GCP) Authorize(ctx context.Context, accessToken string) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://www.googleapis.com/oauth2/v1/tokeninfo?access_token=%s", accessToken), nil)
	if err != nil {
		return err
	}
	cli := &http.Client{}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var tok TokenInfo
	err = json.NewDecoder(resp.Body).Decode(&tok)
	if err != nil {
		return err
	}
	if tok.ExpiresIn < 1 {
		return ErrTokenExpired
	}
	return nil
}

func (g *GCP) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message":"missing token"}`))
			return
		}
		token = strings.TrimPrefix(token, "Bearer ")

		if err := g.Authorize(r.Context(), token); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"unauthorized"}`))
			return
		}

		ctx := context.WithValue(r.Context(), "gcp_access_token", token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

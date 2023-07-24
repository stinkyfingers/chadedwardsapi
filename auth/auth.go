package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Authentication struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	SignedToken string `json:"signedToken"`
}

func JWT() (string, error) {
	key := os.Getenv("JWT_KEY")
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	}).SignedString([]byte(key))
}

func VerifyJWT(tokenString string) error {
	if tokenString == "" {
		return fmt.Errorf("token is empty")
	}
	key := os.Getenv("JWT_KEY")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(key), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	if token.Claims == nil {
		return fmt.Errorf("token is invalid")
	}
	exp, ok := token.Claims.(jwt.MapClaims)["exp"]
	if !ok {
		return fmt.Errorf("token is missing expiration")
	}
	if time.Now().Unix() > int64(exp.(float64)) {
		return fmt.Errorf("token is expired")
	}
	return err
}

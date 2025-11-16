package api

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret []byte

func init() {
	// Берем секрет из переменной окружения или используем дефолтный
	secret := os.Getenv("TODO_JWT_SECRET")
	if secret == "" {
		secret = "todo-secret-key-2024" // для разработки
	}
	jwtSecret = []byte(secret)
}

type Claims struct {
	PasswordHash string `json:"pwd_hash"`
	jwt.RegisteredClaims
}

func generateToken(password string) (string, error) {
	// Создаем простой хеш пароля для payload
	passwordHash := fmt.Sprintf("%x", len(password)) // простой хеш для примера

	expirationTime := time.Now().Add(8 * time.Hour)
	claims := &Claims{
		PasswordHash: passwordHash,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func validateToken(tokenString string) (bool, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return false, err
	}

	if !token.Valid {
		return false, nil
	}

	// Проверяем что токен еще не истек
	if time.Now().After(claims.ExpiresAt.Time) {
		return false, nil
	}

	return true, nil
}

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем требуется ли аутентификация
		password := os.Getenv("TODO_PASSWORD")
		if password == "" {
			// Аутентификация не требуется
			next(w, r)
			return
		}

		// Получаем токен из куки
		var tokenString string
		cookie, err := r.Cookie("token")
		if err == nil {
			tokenString = cookie.Value
		}

		// Валидируем токен
		valid, err := validateToken(tokenString)
		if err != nil || !valid {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		next(w, r)
	})
}

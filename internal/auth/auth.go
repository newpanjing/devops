package auth

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

var jwtSecret = []byte("db-sync-tool-secret-key-change-in-production")

func SetJWTSecret(secret string) {
	jwtSecret = []byte(secret)
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func CreateUser(db *sql.DB, username, password, email string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (username, password_hash, email)
		VALUES (?, ?, ?)
	`, username, hash, email)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func Authenticate(db *sql.DB, username, password string) (*User, error) {
	var user User
	var hash string

	err := db.QueryRow(`
		SELECT id, username, email, password_hash
		FROM users
		WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &user.Email, &hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	if !CheckPasswordHash(password, hash) {
		return nil, fmt.Errorf("invalid password")
	}

	return &user, nil
}

func GenerateToken(user *User) (string, error) {
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func GetUserByID(db *sql.DB, userID int) (*User, error) {
	var user User
	err := db.QueryRow(`
		SELECT id, username, email
		FROM users
		WHERE id = ?
	`, userID).Scan(&user.ID, &user.Username, &user.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}
	return &user, nil
}

func InitializeAdminUser(db *sql.DB) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check users: %w", err)
	}

	if count == 0 {
		return CreateUser(db, "admin", "admin123", "admin@example.com")
	}

	return nil
}

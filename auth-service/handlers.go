package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var phoneRegexp = regexp.MustCompile(`^\+?\d{10,15}$`)
var jwtSecret = []byte("secret")
var refreshMutex sync.Mutex

const (
	AccessTokenExpiry  = time.Hour * 1
	RefreshTokenExpiry = time.Hour * 24 * 7
)

func validatePhone(phone string) bool {
	return phoneRegexp.MatchString(phone)
}

func generateRandomString(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func createTokenPair(user User) (map[string]interface{}, error) {
	accessTokenClaims := jwt.MapClaims{
		"phone_number": user.PhoneNumber,
		"user_id":      user.ID,
		"exp":          time.Now().Add(AccessTokenExpiry).Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString(jwtSecret)
	if err != nil {
		return nil, err
	}

	refreshTokenString, err := generateRandomString(32)
	if err != nil {
		return nil, err
	}

	refreshToken := RefreshToken{
		UserID:    user.ID,
		Token:     refreshTokenString,
		ExpiresAt: time.Now().Add(RefreshTokenExpiry),
	}
	if err := db.Create(&refreshToken).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"access_token":  accessTokenString,
		"refresh_token": refreshTokenString,
		"expires_in":    AccessTokenExpiry.Seconds(),
	}, nil
}

func validateAccessToken(tokenString string) (*jwt.Token, jwt.MapClaims, error) {
	refreshMutex.Lock()
	_, blacklisted := tokenBlacklist[tokenString]
	refreshMutex.Unlock()
	if blacklisted {
		return nil, nil, errors.New("token is blacklisted")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil, errors.New("invalid claims")
	}

	return token, claims, nil
}

func JWTMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Missing or invalid Authorization header"})
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		_, claims, err := validateAccessToken(tokenString)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token: " + err.Error()})
		}

		c.Set("phone_number", claims["phone_number"])
		c.Set("user_id", claims["user_id"])
		return next(c)
	}
}

func register(c echo.Context) error {
	type RegisterRequest struct {
		PhoneNumber string `json:"phone_number"`
		Password    string `json:"password"`
	}
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if !validatePhone(req.PhoneNumber) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid phone format"})
	}
	if len(req.Password) < 6 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Password too short (min 6)"})
	}

	var exists int64
	db.Model(&User{}).Where("phone_number = ?", req.PhoneNumber).Count(&exists)
	if exists > 0 {
		return c.JSON(http.StatusConflict, map[string]string{"error": "User already exists"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
	}

	user := User{
		PhoneNumber:  req.PhoneNumber,
		PasswordHash: string(hashedPassword),
	}
	if err := db.Create(&user).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create user"})
	}

	return c.JSON(http.StatusCreated, map[string]string{"message": "User registered successfully"})
}

func login(c echo.Context) error {
	type LoginRequest struct {
		PhoneNumber string `json:"phone_number"`
		Password    string `json:"password"`
	}
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if !validatePhone(req.PhoneNumber) || len(req.Password) < 6 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid credentials format"})
	}

	var user User
	if err := db.Where("phone_number = ?", req.PhoneNumber).First(&user).Error; err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
	}

	tokenResponse, err := createTokenPair(user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate tokens"})
	}

	return c.JSON(http.StatusOK, tokenResponse)
}

func refreshToken(c echo.Context) error {
	type RefreshRequest struct {
		RefreshToken string `json:"refresh_token"`
	}
	var req RefreshRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	var refreshToken RefreshToken
	if err := db.Where("token = ? AND expires_at > ?", req.RefreshToken, time.Now()).First(&refreshToken).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid or expired refresh token"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
	}

	var user User
	if err := db.First(&user, refreshToken.UserID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "User not found"})
	}

	db.Delete(&refreshToken)

	tokenResponse, err := createTokenPair(user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate tokens"})
	}

	return c.JSON(http.StatusOK, tokenResponse)
}

func logout(c echo.Context) error {
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		accessToken := strings.TrimPrefix(authHeader, "Bearer ")

		refreshMutex.Lock()
		tokenBlacklist[accessToken] = time.Now().Add(AccessTokenExpiry)
		refreshMutex.Unlock()
	}

	type LogoutRequest struct {
		RefreshToken string `json:"refresh_token"`
	}
	var req LogoutRequest
	if err := c.Bind(&req); err == nil && req.RefreshToken != "" {
		db.Where("token = ?", req.RefreshToken).Delete(&RefreshToken{})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

func checkToken(c echo.Context) error {
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Missing or invalid Authorization header"})
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	_, claims, err := validateAccessToken(tokenString)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token: " + err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid":        true,
		"user_id":      claims["user_id"],
		"phone_number": claims["phone_number"],
	})
}

func getProfileProtected(c echo.Context) error {
	phone, ok := c.Get("phone_number").(string)
	if !ok || phone == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}
	var user User
	if err := db.Where("phone_number = ?", phone).First(&user).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":           user.ID,
		"phone_number": user.PhoneNumber,
		"created_at":   user.CreatedAt,
	})
}

func cleanupBlacklist() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		refreshMutex.Lock()
		now := time.Now()
		for token, expiry := range tokenBlacklist {
			if now.After(expiry) {
				delete(tokenBlacklist, token)
			}
		}
		refreshMutex.Unlock()
	}
}

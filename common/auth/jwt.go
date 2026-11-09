package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mvxv0/secure-file-storage/common/httputil"
)

type contextKey string

const UserClaimsKey contextKey = "user_claims"

// Claims predstavlja podatke zapakovane unutar JWT tokena
type Claims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Issued int64  `json:"iat"`
	Expire int64  `json:"exp"`
}

// GenerateToken kreira i potpisuje HS256 JWT token nakon uspesne provere OTP koda
func GenerateToken(secret, userID, email, role string, ttl time.Duration) (string, error) {
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	now := time.Now().Unix()
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		Issued: now,
		Expire: time.Now().Add(ttl).Unix(),
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsBytes)
	unsigned := encodedHeader + "." + encodedClaims

	sig := signHS256(unsigned, secret)
	return unsigned + "." + sig, nil
}

// ValidateToken proverava potpis i rok vazenja JWT tokena
func ValidateToken(tokenStr, secret string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("nevalidan format tokena")
	}

	unsigned := parts[0] + "." + parts[1]
	expectedSig := signHS256(unsigned, secret)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, errors.New("nevalidan potpis tokena")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("neuspesno dekodiranje tokena")
	}

	var claims Claims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, errors.New("nevalidan sadrzaj tokena")
	}

	if time.Now().Unix() > claims.Expire {
		return nil, errors.New("token je istekao")
	}

	return &claims, nil
}

func signHS256(data, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Middleware stiti rute koje zahtevaju da korisnik bude ulogovan (Authorization: Bearer <token>)
func Middleware(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			httputil.WriteError(w, http.StatusUnauthorized, "Nedostaje ili je nevalidan Authorization header")
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := ValidateToken(tokenStr, secret)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}

func GetClaims(r *http.Request) (*Claims, bool) {
	claims, ok := r.Context().Value(UserClaimsKey).(*Claims)
	return claims, ok
}

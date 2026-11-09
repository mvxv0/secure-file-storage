package model

import "time"

type Role string

const (
	RoleRegularUser Role = "RK" // redovan korisnik
	RoleAdmin       Role = "A"  // administrator
)

// User predstavlja dokument korisnika u MongoDB kolekciji 'users'
type User struct {
	ID                string     `json:"id" bson:"_id,omitempty"`
	FirstName         string     `json:"firstName" bson:"first_name"`
	LastName          string     `json:"lastName" bson:"last_name"`
	Email             string     `json:"email" bson:"email"`
	PasswordHash      string     `json:"-" bson:"password_hash"` // json:"-" sprecava slanje hesa lozinke nazad kroz api
	Role              Role       `json:"role" bson:"role"`
	IsVerified        bool       `json:"isVerified" bson:"is_verified"`
	QuotaLimit        int64      `json:"quotaLimit" bson:"quota_limit"`               // maksimalna kvota u bajtovima
	QuotaUsed         int64      `json:"quotaUsed" bson:"quota_used"`                 // trenutno zauzece u bajtovima
	PasswordChangedAt time.Time  `json:"passwordChangedAt" bson:"password_changed_at"` // za proveru starosti sifre (min 1 dan, max 60 dana)
	LockedUntil       *time.Time `json:"lockedUntil,omitempty" bson:"locked_until,omitempty"`
	CreatedAt         time.Time  `json:"createdAt" bson:"created_at"`
	UpdatedAt         time.Time  `json:"updatedAt" bson:"updated_at"`
}

type TokenType string

const (
	TokenVerification  TokenType = "VERIFY_EMAIL"   // token za potvrdu registracije
	TokenOTP           TokenType = "LOGIN_OTP"      // 6-cifreni otp kod za 2fa prijavu
	TokenPasswordReset TokenType = "PASSWORD_RESET" // token za zaboravljenu sifru
	TokenMagicLink     TokenType = "MAGIC_LINK"     // token za povracaj naloga (magic link)
)

// AuthToken cuva privremene kodove i linkove u MongoDB kolekciji 'auth_tokens'
type AuthToken struct {
	ID        string    `json:"id" bson:"_id,omitempty"`
	Email     string    `json:"email" bson:"email"`
	CodeHash  string    `json:"-" bson:"code_hash"`
	Type      TokenType `json:"type" bson:"type"`
	ExpiresAt time.Time `json:"expiresAt" bson:"expires_at"`
	Used      bool      `json:"used" bson:"used"`
	CreatedAt time.Time `json:"createdAt" bson:"created_at"`
}

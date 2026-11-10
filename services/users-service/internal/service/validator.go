package service

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"
)

var (
	ErrEmptyFields       = errors.New("sva polja su obavezna: ime, prezime, email, lozinka i ponovljena lozinka")
	ErrInvalidEmail      = errors.New("format email adrese nije validan")
	ErrPasswordMismatch  = errors.New("lozinka i ponovljena lozinka se ne poklapaju")
	ErrWeakPassword      = errors.New("lozinka nije dovoljno jaka: mora imati bar 8 karaktera, jedno veliko slovo, jedno malo slovo, jedan broj i jedan specijalan karakter")
	ErrInvalidNameLength = errors.New("ime i prezime moraju imati izmedju 2 i 50 karaktera")
)

// ValidateRegistration proverava sva ulazna polja forme za registraciju
func ValidateRegistration(firstName, lastName, email, password, confirmPassword string) error {
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)
	email = strings.TrimSpace(email)

	if firstName == "" || lastName == "" || email == "" || password == "" || confirmPassword == "" {
		return ErrEmptyFields
	}

	if len(firstName) < 2 || len(firstName) > 50 || len(lastName) < 2 || len(lastName) > 50 {
		return ErrInvalidNameLength
	}

	// provera email formata
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrInvalidEmail
	}

	// provera poklapanja lozinki
	if password != confirmPassword {
		return ErrPasswordMismatch
	}

	// provera jacine lozinke
	if err := ValidateStrongPassword(password); err != nil {
		return err
	}

	return nil
}

// ValidateStrongPassword proverava da li lozinka zadovoljava bezbednosna pravila:
// - minimum 8 karaktera
// - bar jedno veliko slovo
// - bar jedno malo slovo
// - bar jedan broj
// - bar jedan specijalan znak
func ValidateStrongPassword(password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool

	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return ErrWeakPassword
	}

	return nil
}


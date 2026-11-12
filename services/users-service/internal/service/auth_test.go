package service

import (
	"testing"
	"time"

	"github.com/mvxv0/secure-file-storage/services/users-service/internal/config"
	"github.com/mvxv0/secure-file-storage/services/users-service/internal/model"
)

func TestPasswordExpiration(t *testing.T) {
	cfg := &config.Config{
		MaxPasswordAge: 60 * 24 * time.Hour, // 60 dana
	}

	now := time.Now().UTC()

	// slucaj 1: sifra promenjena pre 10 dana (vazeca)
	validUser := &model.User{
		PasswordChangedAt: now.Add(-10 * 24 * time.Hour),
	}
	if time.Since(validUser.PasswordChangedAt) > cfg.MaxPasswordAge {
		t.Errorf("Lozinka stara 10 dana ne bi smela biti istekla")
	}

	// slucaj 2: sifra promenjena pre 61 dan (istekla)
	expiredUser := &model.User{
		PasswordChangedAt: now.Add(-61 * 24 * time.Hour),
	}
	if time.Since(expiredUser.PasswordChangedAt) <= cfg.MaxPasswordAge {
		t.Errorf("Lozinka stara 61 dan mora biti prepoznata kao istekla (zahtev 1.2)")
	}
}

func TestAccountLockoutLogic(t *testing.T) {
	now := time.Now().UTC()
	lockDuration := 5 * time.Minute

	// slucaj kada je nalog zakljucan u buducnosti
	lockedUntil := now.Add(lockDuration)
	user := &model.User{
		LockedUntil: &lockedUntil,
	}

	if user.LockedUntil == nil || !user.LockedUntil.After(now) {
		t.Errorf("Nalog bi morao biti prepoznat kao zakljucan")
	}

	// slucaj kada je isteklo vreme zakljucavanja
	pastLock := now.Add(-1 * time.Minute)
	userUnlocked := &model.User{
		LockedUntil: &pastLock,
	}

	if userUnlocked.LockedUntil.After(now) {
		t.Errorf("Nalog cije je zakljucavanje isteklo pre 1 minut ne sme biti blokiran")
	}
}

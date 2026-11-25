package main

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestEnsureTLSCertificate_GeneratesAndReloadsValidX509Cert proverava automatsko generisanje,
// cuvanje na disk, validnost X.509 atributa za 'localhost' i ponovno ucitavanje TLS sertifikata.
func TestEnsureTLSCertificate_GeneratesAndReloadsValidX509Cert(t *testing.T) {
	// osiguravamo da postoji i sertifikat u ../certs/ folderu projekta
	projectCert := filepath.Join("..", "certs", "gateway.crt")
	projectKey := filepath.Join("..", "certs", "gateway.key")
	if _, err := EnsureTLSCertificate(projectCert, projectKey); err != nil {
		t.Fatalf("greska pri generisanju projektnog TLS sertifikata: %v", err)
	}

	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "gateway.crt")
	keyFile := filepath.Join(tmpDir, "gateway.key")

	tlsCfg, err := EnsureTLSCertificate(certFile, keyFile)
	if err != nil {
		t.Fatalf("neocekivana greska pri generisanju TLS sertifikata: %v", err)
	}
	if tlsCfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("ocekivan MinVersion TLS 1.2, dobijeno %d", tlsCfg.MinVersion)
	}
	if len(tlsCfg.Certificates) == 0 || len(tlsCfg.Certificates[0].Certificate) == 0 {
		t.Fatalf("ocekivan bar jedan TLS sertifikat u konfiguraciji")
	}
	if !fileExists(certFile) || !fileExists(keyFile) {
		t.Fatalf("ocekivano da fajlovi %s i %s budu kreirani na disku", certFile, keyFile)
	}

	// provera X.509 polja sertifikata (da li je validan za 'localhost')
	parsedCert, err := x509.ParseCertificate(tlsCfg.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("neuspesno parsiranje X.509 sertifikata: %v", err)
	}
	if err := parsedCert.VerifyHostname("localhost"); err != nil {
		t.Errorf("sertifikat mora biti validan za 'localhost': %v", err)
	}

	// drugi poziv mora da ucita vec postojece fajlove sa diska bez greske
	tlsCfgReloaded, err := EnsureTLSCertificate(certFile, keyFile)
	if err != nil || len(tlsCfgReloaded.Certificates) == 0 {
		t.Fatalf("neuspesno ponovno ucitavanje postojeceg TLS sertifikata: %v", err)
	}
}

// TestSecurityAndMethodMiddleware_BlocksSensitiveQueryParams proverava da middleware blokira
// slanje lozinki, OTP kodova i tokena kroz URL query string, a propusta obicne parametre uz HSTS header.
func TestSecurityAndMethodMiddleware_BlocksSensitiveQueryParams(t *testing.T) {
	handler := SecurityAndMethodMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	blockedURLs := []string{
		"/api/auth/login?email=a@b.com&password=Tajna123!",
		"/api/auth/verify-otp?code=123456",
		"/api/auth/reset-password?token=abc123xyz",
		"/api/users/me?jwt=eyJhbGciOi...",
	}

	for _, u := range blockedURLs {
		req := httptest.NewRequest(http.MethodGet, u, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("URL %s: ocekivan 400 Bad Request zbog senzitivnog query parametra, dobijeno %d", u, rec.Code)
		}
	}

	// bezbedan query parametar (npr. ?folderId=123) mora normalno da prodje uz HSTS header
	safeReq := httptest.NewRequest(http.MethodGet, "/api/files?folderId=folder123", nil)
	safeRec := httptest.NewRecorder()
	handler.ServeHTTP(safeRec, safeReq)
	if safeRec.Code != http.StatusOK {
		t.Errorf("bezbedan query parametar ?folderId=folder123 ne sme biti blokiran, dobijeno %d", safeRec.Code)
	}
	if safeRec.Header().Get("Strict-Transport-Security") == "" {
		t.Errorf("ocekivan Strict-Transport-Security header na HTTPS odgovoru")
	}
}

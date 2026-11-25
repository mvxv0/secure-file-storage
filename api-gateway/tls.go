package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	commonhttp "github.com/mvxv0/secure-file-storage/common/httputil"
)

// lista senzitivnih parametara koji se ne smeju slati kroz URL query string (npr. u GET zahtevima)
var sensitiveQueryParams = []string{
	"password",
	"newpassword",
	"confirmpassword",
	"oldpassword",
	"code",
	"otp",
	"token",
	"secret",
	"jwt",
}

// SecurityAndMethodMiddleware postavlja bezbednosna HTTPS zaglavlja (HSTS) i proverava HTTP metod / query parametre.
// Ako klijent pokusa da posalje senzitivni podatak (lozinku, OTP kod, JWT ili reset token) kroz URL query string,
// zahtev se odbija sa 400 Bad Request kako bi se senzitivni podaci slali iskljucivo u sifrovanom POST/PUT telu.
func SecurityAndMethodMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// HSTS zaglavlje govori browseru da sa ovim serverom komunicira iskljucivo preko HTTPS-a
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")

		// provera da li URL query string sadrzi neki od zabranjenih senzitivnih parametara
		if r.URL.RawQuery != "" {
			queryValues := r.URL.Query()
			for key := range queryValues {
				lowerKey := strings.ToLower(strings.TrimSpace(key))
				for _, forbidden := range sensitiveQueryParams {
					if lowerKey == forbidden {
						log.Printf("[API-GATEWAY] [HTTPS-ZASTITA] Blokiran prenos senzitivnog parametra '%s' kroz URL query na ruti %s %s",
							key, r.Method, r.URL.Path)
						commonhttp.WriteError(
							w,
							http.StatusBadRequest,
							fmt.Sprintf("Senzitivni parametar '%s' se ne sme prenositi kroz URL query string. Koristite HTTPS POST/PUT telo zahteva.", key),
						)
						return
					}
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// EnsureTLSCertificate priprema TLS konfiguraciju za HTTPS server na API Gateway-u:
// 1. Proverava da li na disku vec postoje sertifikat (certs/gateway.crt) i privatni kljuc (certs/gateway.key).
// 2. Ako postoje, ucitava ih i vraca tls.Config sa minimalnom verzijom TLS 1.2.
// 3. Ako ne postoje (npr. nakon git clone ili u novom kontejneru), automatski generise novi self-signed X.509 sertifikat i cuva ga u certs/.
func EnsureTLSCertificate(certPath, keyPath string) (*tls.Config, error) {
	if certPath == "" {
		certPath = filepath.Join("certs", "gateway.crt")
	}
	if keyPath == "" {
		keyPath = filepath.Join("certs", "gateway.key")
	}

	// 1. pokusaj ucitavanja postojeceg sertifikata i kljuca sa diska
	if fileExists(certPath) && fileExists(keyPath) {
		pair, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err == nil {
			log.Printf("[API-GATEWAY] [TLS] Ucitan postojeci TLS sertifikat iz %s i %s", certPath, keyPath)
			return &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{pair},
			}, nil
		}
		log.Printf("[API-GATEWAY] [TLS] Upozorenje: neuspesno ucitavanje postojeceg sertifikata (%v), generisem novi...", err)
	}

	// 2. generisanje novog samopotpisanog (self-signed) X.509 sertifikata i privatnog kljuca
	certPEM, keyPEM, err := generateSelfSignedPEM()
	if err != nil {
		return nil, fmt.Errorf("greska pri generisanju self-signed TLS sertifikata: %w", err)
	}

	// 3. cuvanje generisanog sertifikata (.crt) i kljuca (.key) na disk u certs/ folder
	if err := saveCertFiles(certPath, keyPath, certPEM, keyPEM); err != nil {
		log.Printf("[API-GATEWAY] [TLS] Napomena: sertifikat nije sacuvan na disk (%v), koristi se iz memorije", err)
	} else {
		log.Printf("[API-GATEWAY] [TLS] Generisan i sacuvan self-signed TLS sertifikat: %s, %s", certPath, keyPath)
	}

	// 4. kreiranje TLS para iz PEM bajtova i podesavanje minimalne verzije protokola na TLS 1.2
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("greska pri parsiranju generisanog TLS para: %w", err)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{pair},
	}, nil
}

// generateSelfSignedPEM kreira samopotpisani X.509 sertifikat i ECDSA privatni kljuc u PEM formatu:
// koristi ECDSA algoritam sa eliptickom krivom P-256 za generisanje asimetricnog para kljuceva
// postavlja SAN (Subject Alternative Names) za localhost i 127.0.0.1
// potpisuje sertifikat sopstvenim privatnim kljucem (self-signed) sa rokom vazenja od godinu dana
func generateSelfSignedPEM() ([]byte, []byte, error) {
	// generisanje ECDSA P-256 privatnog i javnog kljuca
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	// generisanje nasumicnog 128-bitnog serijskog broja sertifikata
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	// definisanje X.509 sablona sertifikata
	now := time.Now().UTC()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Secure File Storage - FTN"},
			CommonName:   "localhost",
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(365 * 24 * time.Hour), // vazi 1 godinu
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true, // omogucava uvoz u Trusted Root Certification Authorities u browseru
		DNSNames:              []string{"localhost", "api-gateway", "gdrive-api-gateway"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	// samopotpisivanje sertifikata (template je i subjekt i izdavalac/parent)
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	// enkodovanje javnog sertifikata u tekstualni PEM format (-----BEGIN CERTIFICATE-----)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	// serijalizacija i enkodovanje privatnog kljuca u PEM format (-----BEGIN EC PRIVATE KEY-----)
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return certPEM, keyPEM, nil
}

// saveCertFiles kreira certs/ direktorijum ako ne postoji i upisuje .crt (0644) i .key (0600) fajlove na disk
func saveCertFiles(certPath, keyPath string, certPEM, keyPEM []byte) error {
	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return err
	}
	// 0600 dozvole za privatni kljuc (samo vlasnik moze da ga cita)
	return os.WriteFile(keyPath, keyPEM, 0600)
}

// fileExists proverava da li fajl postoji na zadatoj putanji i nije direktorijum
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

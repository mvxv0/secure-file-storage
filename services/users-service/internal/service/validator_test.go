package service

import "testing"

func TestValidateStrongPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"Prekratka lozinka", "Ab1!", true},
		{"Nema veliko slovo", "lozinka123!", true},
		{"Nema malo slovo", "LOZINKA123!", true},
		{"Nema broj", "Lozinka!@#", true},
		{"Nema specijalan znak", "Lozinka1234", true},
		{"Ispravna jaka lozinka", "SigurnaLozinka123!", false},
		{"Druga ispravna jaka lozinka", "MojNalog#2026", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStrongPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStrongPassword(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
			}
		})
	}
}


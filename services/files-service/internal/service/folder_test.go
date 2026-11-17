package service

import (
	"strings"
	"testing"
)

func TestFolderValidation(t *testing.T) {
	tests := []struct {
		name      string
		inputName string
		wantErr   bool
	}{
		{"Prazan naziv", "", true},
		{"Samo razmaci", "   ", true},
		{"Sadrzi kosu crtu", "projekat/diplomski", true},
		{"Sadrzi backslash", "projekat\\zadatak", true},
		{"Tacka kao ime (path traversal)", ".", true},
		{"Dve tacke kao ime (path traversal)", "..", true},
		{"Predugacak naziv", strings.Repeat("a", 101), true},
		{"Ispravan korenski folder", "Dokumenti", false},
		{"Ispravan podfolder sa razmakom i brojem", "Predmet SOA 2026", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned := strings.TrimSpace(tt.inputName)
			isInvalid := cleaned == "" || len(cleaned) > 100 || strings.ContainsAny(cleaned, "/\\") || cleaned == "." || cleaned == ".."
			if isInvalid != tt.wantErr {
				t.Errorf("Folder validation for %q = %v, want %v", tt.inputName, isInvalid, tt.wantErr)
			}
		})
	}
}


package service

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateFileInput(t *testing.T) {
	tests := []struct {
		name      string
		fileName  string
		content   []byte
		wantErr   error
	}{
		{
			name:     "Ispravna tekstualna datoteka",
			fileName: "beleške.txt",
			content:  []byte("Ovo je test sadrzaj datoteke"),
			wantErr:  nil,
		},
		{
			name:     "Ispravna PDF datoteka",
			fileName: "specifikacija.pdf",
			content:  []byte("%PDF-1.4 test"),
			wantErr:  nil,
		},
		{
			name:     "Prazan naziv datoteke",
			fileName: "   ",
			content:  []byte("sadrzaj"),
			wantErr:  ErrInvalidFileName,
		},
		{
			name:     "Path traversal u nazivu",
			fileName: "../tajna.txt",
			content:  []byte("sadrzaj"),
			wantErr:  ErrInvalidFileName,
		},
		{
			name:     "Prazan sadrzaj (0 bajtova)",
			fileName: "prazan.txt",
			content:  []byte{},
			wantErr:  ErrEmptyFileContent,
		},
		{
			name:     "Zabranjena izvršna ekstenzija (.exe)",
			fileName: "malware.exe",
			content:  []byte("MZ..."),
			wantErr:  ErrDisallowedFileExt,
		},
		{
			name:     "Zabranjena skripta (.sh)",
			fileName: "skripta.sh",
			content:  []byte("#!/bin/bash"),
			wantErr:  ErrDisallowedFileExt,
		},
		{
			name:     "Predugacak naziv datoteke",
			fileName: strings.Repeat("a", 155) + ".txt",
			content:  []byte("sadrzaj"),
			wantErr:  ErrInvalidFileName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ValidateFileInput(tt.fileName, tt.content)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("neocekivana greska: %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("dobijena greska %v, ocekivano %v", err, tt.wantErr)
			}
		})
	}
}

func TestCalculateChecksum(t *testing.T) {
	c1 := CalculateChecksum([]byte("verzija 1"))
	c2 := CalculateChecksum([]byte("verzija 2"))
	if c1 == "" || len(c1) != 64 {
		t.Fatalf("neispravan SHA-256 checksum: %s", c1)
	}
	if c1 == c2 {
		t.Fatalf("checksum za razlicit sadrzaj mora biti razlicit")
	}
}

func TestValidateFileNameForRename(t *testing.T) {
	if _, err := ValidateFileName("izvestaj_2026.pdf"); err != nil {
		t.Fatalf("ocekivan validan naziv datoteke, dobijena greska: %v", err)
	}
	if _, err := ValidateFileName("../hack.txt"); !errors.Is(err, ErrInvalidFileName) {
		t.Fatalf("ocekivana greska ErrInvalidFileName za path traversal, dobijeno: %v", err)
	}
	if _, err := ValidateFileName("skripta.exe"); !errors.Is(err, ErrDisallowedFileExt) {
		t.Fatalf("ocekivana greska ErrDisallowedFileExt za .exe prilikom preimenovanja, dobijeno: %v", err)
	}
}


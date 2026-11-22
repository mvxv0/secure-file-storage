package service

import (
	"errors"
	"testing"
)

func TestValidateQuotaRequest(t *testing.T) {
	const mb = 1024 * 1024
	current := int64(100 * mb)

	tests := []struct {
		name      string
		requested int64
		wantErr   error
	}{
		{
			name:      "Validno povecanje na 250 MB",
			requested: 250 * mb,
			wantErr:   nil,
		},
		{
			name:      "Ista vrednost kao trenutna kvota",
			requested: 100 * mb,
			wantErr:   ErrInvalidRequestedQuota,
		},
		{
			name:      "Manja vrednost od trenutne kvote",
			requested: 50 * mb,
			wantErr:   ErrInvalidRequestedQuota,
		},
		{
			name:      "Nula ili negativna vrednost",
			requested: 0,
			wantErr:   ErrInvalidRequestedQuota,
		},
		{
			name:      "Preko maksimalnog limita od 10 GB",
			requested: MaxAllowedStorageQuota + 1,
			wantErr:   ErrQuotaLimitTooHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQuotaRequest(current, tt.requested)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("neocekivana greska: %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("dobijena greska %v, ocekivano %v", err, tt.wantErr)
			}
		})
	}
}


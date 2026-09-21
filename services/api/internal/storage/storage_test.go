package storage_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/doc-intel/api/internal/storage"
)

func TestStorageKey(t *testing.T) {
	id := uuid.MustParse("11111111-2222-3333-4444-555555555555")

	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "pdf file",
			filename: "invoice.pdf",
			want:     "documents/11111111-2222-3333-4444-555555555555/original.pdf",
		},
		{
			name:     "png file",
			filename: "cheque.png",
			want:     "documents/11111111-2222-3333-4444-555555555555/original.png",
		},
		{
			name:     "jpeg file",
			filename: "scan.jpeg",
			want:     "documents/11111111-2222-3333-4444-555555555555/original.jpeg",
		},
		{
			name:     "uppercase extension",
			filename: "DOCUMENT.PDF",
			want:     "documents/11111111-2222-3333-4444-555555555555/original.PDF",
		},
		{
			name:     "no extension",
			filename: "nodot",
			want:     "documents/11111111-2222-3333-4444-555555555555/original",
		},
		{
			name:     "filename with dots",
			filename: "my.invoice.v2.pdf",
			want:     "documents/11111111-2222-3333-4444-555555555555/original.pdf",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := storage.StorageKey(id, tc.filename)
			if got != tc.want {
				t.Errorf("StorageKey(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

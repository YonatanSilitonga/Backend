package mobile_api

import (
	"errors"
	"testing"
)

func TestValidateRunningRitaseUpdate(t *testing.T) {
	cur := ritaseHeaderSnapshot{
		IDDriver:    59,
		IDKendaraan: 22,
		RitaseKe:    3,
		Status:      "berjalan",
		JenisRitase: "outgoing",
	}

	tests := []struct {
		name    string
		req     UpdateRitaseRequest
		wantErr bool
	}{
		{
			name: "header sama tidak dianggap perubahan",
			req: UpdateRitaseRequest{
				IDDriver:    59,
				IDKendaraan: 22,
				RitaseKe:    3,
				Status:      "berjalan",
				JenisRitase: "outgoing",
			},
			wantErr: false,
		},
		{
			name:    "header kosong tidak dianggap perubahan",
			req:     UpdateRitaseRequest{},
			wantErr: false,
		},
		{
			name: "driver berubah ditolak",
			req: UpdateRitaseRequest{
				IDDriver: 60,
			},
			wantErr: true,
		},
		{
			name: "kendaraan berubah ditolak",
			req: UpdateRitaseRequest{
				IDKendaraan: 23,
			},
			wantErr: true,
		},
		{
			name: "ritase ke berubah ditolak",
			req: UpdateRitaseRequest{
				RitaseKe: 4,
			},
			wantErr: true,
		},
		{
			name: "status berubah ditolak",
			req: UpdateRitaseRequest{
				Status: "selesai",
			},
			wantErr: true,
		},
		{
			name: "jenis ritase berubah ditolak",
			req: UpdateRitaseRequest{
				JenisRitase: "incoming",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRunningRitaseUpdate(tt.req, cur)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, errRunningRitaseHeaderChanged) {
					t.Fatalf("expected errRunningRitaseHeaderChanged, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

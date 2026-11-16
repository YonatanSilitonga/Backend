package utilisasi

import "testing"

func TestResolveKoliByGateway(t *testing.T) {
	tests := []struct {
		name    string
		trip    tripRow
		want    int
		wantSrc string
	}{
		{
			name:    "jkt confirmation",
			trip:    tripRow{GatewayAkhir: "JKT", KonfirmasiKoli: 74, KonfirmasiKoliJKT: 52, KonfirmasiKoliSEG: 22},
			want:    52,
			wantSrc: "konfirmasi",
		},
		{
			name:    "seg confirmation includes btn",
			trip:    tripRow{GatewayAkhir: "SEG", KonfirmasiKoli: 74, KonfirmasiKoliJKT: 52, KonfirmasiKoliSEG: 22},
			want:    22,
			wantSrc: "konfirmasi",
		},
		{
			name:    "jkt event regional preferred",
			trip:    tripRow{GatewayAkhir: "JKT", EventKoli: 74, EventKoliJKT: 53, EventKoliSEG: 21, KonfirmasiKoli: 74, KonfirmasiKoliJKT: 52, KonfirmasiKoliSEG: 22},
			want:    53,
			wantSrc: "event",
		},
		{
			name:    "seg event regional preferred",
			trip:    tripRow{GatewayAkhir: "SEG", EventKoli: 74, EventKoliJKT: 53, EventKoliSEG: 21, KonfirmasiKoli: 74, KonfirmasiKoliJKT: 52, KonfirmasiKoliSEG: 22},
			want:    21,
			wantSrc: "event",
		},
		{
			name:    "jkt falls back to confirmation when event has no regional",
			trip:    tripRow{GatewayAkhir: "JKT", EventKoli: 74, KonfirmasiKoli: 74, KonfirmasiKoliJKT: 52, KonfirmasiKoliSEG: 22},
			want:    52,
			wantSrc: "konfirmasi",
		},
		{
			name:    "seg falls back to confirmation when event has no regional",
			trip:    tripRow{GatewayAkhir: "SEG", EventKoli: 74, KonfirmasiKoli: 74, KonfirmasiKoliJKT: 52, KonfirmasiKoliSEG: 22},
			want:    22,
			wantSrc: "konfirmasi",
		},
		{
			name:    "jkt with only seg koli is zero",
			trip:    tripRow{GatewayAkhir: "JKT", EventKoli: 22, EventKoliSEG: 22},
			want:    0,
			wantSrc: "event",
		},
		{
			name:    "seg with only jkt koli is zero",
			trip:    tripRow{GatewayAkhir: "SEG", KonfirmasiKoli: 52, KonfirmasiKoliJKT: 52},
			want:    0,
			wantSrc: "konfirmasi",
		},
		{
			name:    "jkt falls back to total when no regional data",
			trip:    tripRow{GatewayAkhir: "JKT", EventKoli: 80},
			want:    80,
			wantSrc: "event",
		},
		{
			name:    "seg falls back to total confirmation when no regional data",
			trip:    tripRow{GatewayAkhir: "SEG", KonfirmasiKoli: 70},
			want:    70,
			wantSrc: "konfirmasi",
		},
		{
			name:    "unknown gateway uses event total",
			trip:    tripRow{EventKoli: 74, KonfirmasiKoli: 70},
			want:    74,
			wantSrc: "event",
		},
		{
			name:    "unknown gateway uses confirmation total",
			trip:    tripRow{KonfirmasiKoli: 70, RitaseKoli: 65},
			want:    70,
			wantSrc: "konfirmasi",
		},
		{
			name:    "unknown gateway uses ritase total",
			trip:    tripRow{RitaseKoli: 65},
			want:    65,
			wantSrc: "ritase",
		},
		{
			name:    "no data",
			trip:    tripRow{GatewayAkhir: "JKT"},
			want:    0,
			wantSrc: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, src := resolveKoli(tt.trip)
			if got != tt.want || src != tt.wantSrc {
				t.Fatalf("resolveKoli() = (%d, %q), want (%d, %q)", got, src, tt.want, tt.wantSrc)
			}
		})
	}
}

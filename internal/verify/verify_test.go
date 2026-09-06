package verify

import (
	"testing"
)

func TestParseChecksumsStrict(t *testing.T) {
	hash1 := "1111111111111111111111111111111111111111111111111111111111111111"
	hash2 := "2222222222222222222222222222222222222222222222222222222222222222"
	hash3 := "3333333333333333333333333333333333333333333333333333333333333333"
	hash4 := "4444444444444444444444444444444444444444444444444444444444444444"
	hash5 := "5555555555555555555555555555555555555555555555555555555555555555"

	content := `
` + hash1 + `  nuclei_3.0_linux_amd64.zip
` + hash2 + ` *nuclei_3.0_windows_amd64.zip
SHA256 (nuclei_3.0_darwin_amd64.zip) = ` + hash3 + `
SHA256(nuclei_3.0_darwin_arm64.zip)= ` + hash4 + `
invalid line
` + hash5 + `  my-nuclei_3.0_linux_amd64.zip.backup
` + hash1 + `  duplicate_ambiguous.zip
` + hash2 + `  duplicate_ambiguous.zip
`

	tests := []struct {
		name       string
		target     string
		want       string
		wantErr    bool
	}{
		{
			name:   "coreutils format with spaces",
			target: "nuclei_3.0_linux_amd64.zip",
			want:   hash1,
			wantErr: false,
		},
		{
			name:   "coreutils format with asterisk",
			target: "nuclei_3.0_windows_amd64.zip",
			want:   hash2,
			wantErr: false,
		},
		{
			name:   "openssl format with space",
			target: "nuclei_3.0_darwin_amd64.zip",
			want:   hash3,
			wantErr: false,
		},
		{
			name:   "openssl format without space",
			target: "nuclei_3.0_darwin_arm64.zip",
			want:   hash4,
			wantErr: false,
		},
		{
			name:   "reject substring match",
			target: "nuclei_3.0_linux_amd64.zip.backup", // Should only match the exact one
			want:   "",
			wantErr: true,
		},
		{
			name:   "ambiguous matches fail",
			target: "duplicate_ambiguous.zip",
			want:   "",
			wantErr: true,
		},
		{
			name:   "not found fails",
			target: "non_existent.zip",
			want:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChecksumsStrict(content, tt.target, "sha256")
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseChecksumsStrict() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseChecksumsStrict() got = %v, want %v", got, tt.want)
			}
		})
	}
}

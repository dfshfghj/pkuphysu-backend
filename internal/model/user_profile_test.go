package model

import (
	"strings"
	"testing"
)

func TestValidateUserProfileContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "empty content is allowed",
			content: "",
			wantErr: false,
		},
		{
			name:    "content at the limit is allowed",
			content: strings.Repeat("a", MaxUserProfileContentLength),
			wantErr: false,
		},
		{
			name:    "content over the limit is rejected",
			content: strings.Repeat("a", MaxUserProfileContentLength+1),
			wantErr: true,
		},
		{
			name: "length counts runes instead of bytes",
			content: strings.Repeat("京", MaxUserProfileContentLength),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserProfileContent(tt.content)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateUserProfileContent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

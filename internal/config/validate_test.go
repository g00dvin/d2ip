package config

import "testing"

func TestValidatePolicies(t *testing.T) {
	tests := []struct {
		name     string
		policies []PolicyConfig
		wantErr  bool
	}{
		{
			name: "valid single policy",
			policies: []PolicyConfig{{
				Name:       "streaming",
				Enabled:    true,
				Categories: []string{"geosite:netflix"},
				Backend:    BackendIProute2,
				TableID:    200,
				Iface:      "eth1",
			}},
		},
		{
			name: "missing name",
			policies: []PolicyConfig{{
				Enabled:    true,
				Categories: []string{"geosite:netflix"},
				Backend:    BackendIProute2,
			}},
			wantErr: true,
		},
		{
			name: "duplicate table_id",
			policies: []PolicyConfig{
				{Name: "a", Enabled: true, Categories: []string{"geosite:x"}, Backend: BackendIProute2, TableID: 200, Iface: "eth0"},
				{Name: "b", Enabled: true, Categories: []string{"geosite:y"}, Backend: BackendIProute2, TableID: 200, Iface: "eth1"},
			},
			wantErr: true,
		},
		{
			name: "disabled policy needs no categories",
			policies: []PolicyConfig{{
				Name:    "disabled",
				Enabled: false,
				Backend: BackendNone,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validatePolicies(tt.policies)
			if tt.wantErr && len(errs) == 0 {
				t.Fatal("expected errors, got none")
			}
			if !tt.wantErr && len(errs) > 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}
		})
	}
}

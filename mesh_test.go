package nanomesh

import "testing"

func TestParseMeshConfig(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   MeshConfig
	}{
		{"defaults", map[string]string{}, MeshConfig{NetworkName: "swarmex", NetworkSecret: ""}},
		{"custom", map[string]string{labelNetwork: "mynet", labelSecret: "s3cret"}, MeshConfig{NetworkName: "mynet", NetworkSecret: "s3cret"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMeshConfig(tt.labels)
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

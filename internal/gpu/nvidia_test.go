package gpu

import "testing"

func TestParseGPUGeneration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected GPUGeneration
	}{
		{"RTX 2080", "NVIDIA GeForce RTX 2080 Ti", GPUGenerationTuring},
		{"RTX 3090", "NVIDIA GeForce RTX 3090", GPUGenerationAmpere},
		{"RTX 4070 Ti", "NVIDIA GeForce RTX 4070 Ti", GPUGenerationAdaLovelace},
		{"RTX 4090", "NVIDIA GeForce RTX 4090", GPUGenerationAdaLovelace},
		{"RTX 5090", "NVIDIA GeForce RTX 5090", GPUGenerationBlackwell},
		{"Unknown", "NVIDIA GeForce GTX 1080", GPUGenerationUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGPUGeneration(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGPUGenerationSupportsFP8(t *testing.T) {
	tests := []struct {
		gen      GPUGeneration
		expected bool
	}{
		{GPUGenerationUnknown, false},
		{GPUGenerationTuring, false},
		{GPUGenerationAmpere, false},
		{GPUGenerationAdaLovelace, true},
		{GPUGenerationBlackwell, true},
	}

	for _, tt := range tests {
		t.Run(tt.gen.String(), func(t *testing.T) {
			if tt.gen.SupportsFP8() != tt.expected {
				t.Errorf("expected SupportsFP8()=%v for %s", tt.expected, tt.gen)
			}
		})
	}
}

package config

import "testing"

func TestSnowAccumPerLanding(t *testing.T) {
	tests := []struct {
		rate float64
		want int
	}{
		{0, 1},
		{-1, 1},
		{1, 1},
		{1.4, 1},
		{1.5, 2},
		{3, 3},
		{8, 8},
		{8.4, 8},
		{99, 8},
	}
	for _, tt := range tests {
		if got := SnowAccumPerLanding(tt.rate); got != tt.want {
			t.Errorf("SnowAccumPerLanding(%v) = %d, want %d", tt.rate, got, tt.want)
		}
	}
}

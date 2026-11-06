package repository

import (
	"math"
	"testing"
)

func TestMemoryVectorValidationRejectsUnknownSpaceAndInvalidVectors(t *testing.T) {
	vector := make([]float64, 3072)
	vector[0] = 1
	if err := validateMemoryVector(vector, "model:padded:3072"); err != nil {
		t.Fatal(err)
	}
	for _, space := range []string{"", " ", "legacy:unknown"} {
		if err := validateMemoryVector(vector, space); err == nil {
			t.Errorf("invalid space accepted: %q", space)
		}
	}
	for _, values := range [][]float64{nil, make([]float64, 384), make([]float64, 3072)} {
		if err := validateMemoryVector(values, "valid"); err == nil {
			t.Fatal("invalid vector accepted")
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.MaxFloat64} {
		vector[0] = value
		if err := validateMemoryVector(vector, "valid"); err == nil {
			t.Fatal("non-finite vector accepted")
		}
	}
}

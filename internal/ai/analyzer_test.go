package ai

import (
	"testing"
)

func TestValidateColors_Valid(t *testing.T) {
	colors := map[string]any{
		"gradient_from":  "#FF8800",
		"gradient_to":    "#0088FF",
		"gradient_angle": float64(135),
	}

	result, err := ValidateColors(colors)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["gradient_from"] != "#FF8800" {
		t.Errorf("gradient_from = %v, want #FF8800", result["gradient_from"])
	}
	if result["gradient_angle"] != 135 {
		t.Errorf("gradient_angle = %v, want 135", result["gradient_angle"])
	}
}

func TestValidateColors_NullValues(t *testing.T) {
	colors := map[string]any{
		"gradient_from":  nil,
		"gradient_to":    "#0088FF",
		"gradient_angle": float64(135),
	}

	_, err := ValidateColors(colors)
	if err == nil {
		t.Fatal("expected error for null gradient_from")
	}
}

func TestValidateColors_MissingField(t *testing.T) {
	colors := map[string]any{
		"gradient_from": "#FF8800",
		"gradient_to":   "#0088FF",
	}

	_, err := ValidateColors(colors)
	if err == nil {
		t.Fatal("expected error for missing gradient_angle")
	}
}

func TestValidateColors_FixesMissingHash(t *testing.T) {
	colors := map[string]any{
		"gradient_from":  "FF8800",
		"gradient_to":    "#0088FF",
		"gradient_angle": float64(45),
	}

	result, err := ValidateColors(colors)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["gradient_from"] != "#FF8800" {
		t.Errorf("gradient_from = %v, want #FF8800 (with # added)", result["gradient_from"])
	}
}

func TestValidateColors_StringAngle(t *testing.T) {
	colors := map[string]any{
		"gradient_from":  "#FF8800",
		"gradient_to":    "#0088FF",
		"gradient_angle": "135",
	}

	result, err := ValidateColors(colors)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["gradient_angle"] != 135 {
		t.Errorf("gradient_angle = %v, want 135", result["gradient_angle"])
	}
}

func TestValidateColors_InvalidHex(t *testing.T) {
	colors := map[string]any{
		"gradient_from":  "#ZZZZZZ",
		"gradient_to":    "#0088FF",
		"gradient_angle": float64(135),
	}

	_, err := ValidateColors(colors)
	if err == nil {
		t.Fatal("expected error for invalid hex color")
	}
}

func TestValidateColors_NormalizesCase(t *testing.T) {
	colors := map[string]any{
		"gradient_from":  "#ffb968",
		"gradient_to":    "#e85d75",
		"gradient_angle": float64(135),
	}

	result, err := ValidateColors(colors)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["gradient_from"] != "#FFB968" {
		t.Errorf("gradient_from = %v, want #FFB968", result["gradient_from"])
	}
}

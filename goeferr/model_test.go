package goeferr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
)

func TestInvalidModelDetail(t *testing.T) {
	err := fmt.Errorf("freeze: %w", goeferr.NewInvalidModel())
	if !errors.Is(err, goeferr.ErrInvalidModel) {
		t.Fatalf("wrapped error does not match ErrInvalidModel: %v", err)
	}

	var detail *goeferr.ErrorDetail
	if !errors.As(err, &detail) {
		t.Fatalf("wrapped error has no structured detail: %v", err)
	}
	if detail.ID() != goeferr.InvalidModelID || detail.ID() != "GOEF-MODEL-0001" {
		t.Errorf("unexpected model error ID: %q", detail.ID())
	}
	if detail.Phase() != "validate-model" || detail.RetryClass() != "never" || detail.Outcome() != "no_commit_attempted" {
		t.Errorf("unexpected model error policy: %q, %q, %q", detail.Phase(), detail.RetryClass(), detail.Outcome())
	}
	const message = "Model validation failed."
	if detail.Error() != message || goeferr.ErrInvalidModel.Error() != message {
		t.Errorf("unexpected public message: %q", detail.Error())
	}
}

func TestUninitializedDetailHasNoClassification(t *testing.T) {
	for _, detail := range []*goeferr.ErrorDetail{nil, {}} {
		if errors.Is(detail, goeferr.ErrInvalidModel) || errors.Unwrap(detail) != nil {
			t.Errorf("uninitialized detail reported a model validation error: %v", detail)
		}
		if detail.ID() != "" || detail.Phase() != "" || detail.RetryClass() != "" || detail.Outcome() != "" {
			t.Errorf("uninitialized detail reported error metadata: %v", detail)
		}
		if got := detail.Error(); got == "" || got == goeferr.ErrInvalidModel.Error() {
			t.Errorf("unexpected uninitialized detail message: %q", got)
		}
	}
}

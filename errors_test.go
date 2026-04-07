package agentmem

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrors(t *testing.T) {
	t.Run("ErrNotFound", func(t *testing.T) {
		if ErrNotFound.Error() != "entry not found" {
			t.Errorf("unexpected message: %s", ErrNotFound.Error())
		}
	})

	t.Run("ErrStoreClosed", func(t *testing.T) {
		if ErrStoreClosed.Error() != "store is closed" {
			t.Errorf("unexpected message: %s", ErrStoreClosed.Error())
		}
	})

	t.Run("ErrEmptyKey", func(t *testing.T) {
		if ErrEmptyKey.Error() != "key must not be empty" {
			t.Errorf("unexpected message: %s", ErrEmptyKey.Error())
		}
	})

	t.Run("ErrNilValue", func(t *testing.T) {
		if ErrNilValue.Error() != "value must not be nil" {
			t.Errorf("unexpected message: %s", ErrNilValue.Error())
		}
	})

	t.Run("errors are wrappable", func(t *testing.T) {
		wrapped := fmt.Errorf("op failed: %w", ErrNotFound)
		if !errors.Is(wrapped, ErrNotFound) {
			t.Error("wrapped error should match ErrNotFound")
		}
	})

	t.Run("errors are distinct", func(t *testing.T) {
		if errors.Is(ErrNotFound, ErrStoreClosed) {
			t.Error("ErrNotFound should not equal ErrStoreClosed")
		}
		if errors.Is(ErrEmptyKey, ErrNilValue) {
			t.Error("ErrEmptyKey should not equal ErrNilValue")
		}
	})
}

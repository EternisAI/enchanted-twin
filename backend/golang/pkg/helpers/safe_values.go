package helpers

import (
	"errors"
	"time"
)

func SafeValue[T any](value *T) T {
	if value == nil {
		return *new(T)
	}
	return *value
}

func Ptr[T any](value T) *T {
	return &value
}

func CastToType[T any](val any) (T, error) {
	typedVal, ok := val.(T)
	if !ok {
		return typedVal, errors.New("value is not of type %T")
	}
	return typedVal, nil
}

// TimeToString converts a time.Time to a string pointer using the provided format, or the default RFC3339 format if none is provided.
// If the time is zero, it returns nil.
func TimeToStringPtr(t time.Time, fs ...string) *string {
	var out *string

	f := time.RFC3339
	if len(fs) > 0 {
		f = fs[0]
	}
	if !t.IsZero() {
		res := t.Format(f)
		out = &res
	}
	return out
}

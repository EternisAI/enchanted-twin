package helpers

import "errors"

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

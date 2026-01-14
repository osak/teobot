package chatgpt

/*
import (
	"fmt"
)

type jsonWrapper struct {
	raw map[string]interface{}
}

func (w *jsonWrapper) get(name string) (val any, ok bool) {
	val, ok = w.raw[name]
	return val, ok
}

func get[T any](w *jsonWrapper, name string) (*T, error) {
	val, ok := w.raw[name]
	if !ok {
		return nil, fmt.Errorf("field %q not found", name)
	}
	typed, ok := val.(T)
	if !ok {
		return nil, fmt.Errorf("value of %q is not T", name)
	}
	return &typed, nil
}

func mustGet[T any](w *jsonWrapper, name string) *T {
	val, err := get[T](w, name)
	if err != nil {
		panic(err)
	}
	return val
}

type RawOutputText struct {
	*jsonWrapper
	Text string
}

func (r *RawOutputText) Text() string {
	return *mustGet[string](r.jsonWrapper, "text")
}

*/

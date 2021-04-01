package jsonextract

import (
	"encoding/json"
	"errors"
	"io"
	"sort"
)

// Unmarshal returns a callback function that can be used with the Objects method for decoding one
// element. After verify returns true, the object will no longer be changed.
//
// Please note that any Unmarshal errors will be ignored, which means that if you don't pass a pointer
// or your struct field types don't match the ones in the data, you will not be notified about the error.
func Unmarshal(pointer interface{}, verify func() bool) JSONCallback {
	return func(b []byte) error {

		err := json.Unmarshal(b, pointer)
		if err != nil {
			return nil
		}

		// If true, we never change the object again
		if verify() {
			return ErrStop
		}

		return nil
	}
}

// ObjectOption defines filters and callbacks for the Object method
type ObjectOption struct {
	// Keys defines a filter for objects. Only objects where these keys are present will be passed to Callback.
	// If this is not set, all objects will be passed to the callback.
	Keys []string

	// Callback receives JSON bytes for all objects that have all keys defined by Keys.
	// Returning ErrStop will stop extraction without error. Other errors will be returned.
	Callback JSONCallback

	// Required sets whether ErrCallbackNeverCalled should be returned if the callback function for this ObjectOption is not called
	Required bool
}

func (s *ObjectOption) match(m map[string]rawMessageNoCopy) bool {
	for _, k := range s.Keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

// ErrCallbackNeverCalled is returned from the Objects method if the callback of a required ObjectOption was never satisfied,
// which means that the callback never returned ErrStop.
var ErrCallbackNeverCalled = errors.New("callback never called")

// Objects extracts all nested objects and passes them to appropriate callback functions.
// You can define which keys must be present for an object to be passed to your function.
//
// This method will check not just top-level object keys, but also those of all child objects.
//
// If multiple options would match, only the first one will be processed. This allows you to cascade options
// to first extract objects with the most keys, then those with less (which is useful if there are overlapping keys).
//
// If a required option is not matched, ErrCallbackNeverCalled will be returned.
//
// Arrays/Slices will not cause a callback as they don't have keys, but objects in them will be matched.
func Objects(r io.Reader, o []ObjectOption) (err error) {

	var (
		satisfiedCallbacks = make(map[int]bool)
		satisfiedCount     int

		keyFunc func(b []byte) error
	)

	keyFunc = func(b []byte) (err error) {
		if b[0] == '[' {
			// Decode the array
			var arr []rawMessageNoCopy

			err = json.Unmarshal(b, &arr)
			if err != nil {
				return
			}

			// Now walk through all elements and check them using this same function
			for _, elem := range arr {
				err = keyFunc(elem)
				if err != nil {
					return
				}
			}
		} else if b[0] == '{' {
			var m map[string]rawMessageNoCopy

			err = json.Unmarshal(b, &m)
			if err != nil {
				return
			}

			// Match the first option that is good for this struct
			for i, opt := range o {
				if satisfiedCallbacks[i] {
					continue
				}

				if opt.match(m) {
					oerr := opt.Callback(b)
					if oerr == ErrStop {
						// Mark this callback function as done
						satisfiedCallbacks[i] = true
						satisfiedCount++

						// When all options are satisfied, there's no point in continuing
						if satisfiedCount == len(o) {
							return ErrStop
						}

						// Make sure we don't terminate too early
						oerr = nil
					} else if oerr != nil {
						return oerr
					}

					// Since only the first option that matches should be called
					break
				}
			}

			// Go through map alphabetically by sorting keys first, that
			// makes the output more deterministic
			var keys = make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}

			sort.Strings(keys)

			for _, key := range keys {
				err = keyFunc(m[key])
				if err != nil {
					return
				}
			}
		}

		return nil
	}

	err = Reader(r, keyFunc)

	// Only check required callbacks if there are no other errors
	if err == nil && satisfiedCount != len(o) {
		for i, oo := range o {
			if oo.Required {
				// If the callback of a required option was never satisfied, we return an error
				if _, ok := satisfiedCallbacks[i]; !ok {
					err = ErrCallbackNeverCalled
					break
				}
			}
		}
	}

	return
}

// rawMessageNoCopy is like json.RawMessage, except that it doesn't make a full copy
type rawMessageNoCopy []byte

// compile error if we don't implement Unmarshaler
var _ json.Unmarshaler = (*rawMessageNoCopy)(nil)

// UnmarshalJSON sets *m to data, implements json.Unmarshaler
func (m *rawMessageNoCopy) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.New("rawMessageNoCopy: UnmarshalJSON on nil pointer")
	}
	// this should copy the slice header, but not the underlying data
	*m = data

	return nil
}

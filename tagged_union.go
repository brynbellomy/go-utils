package utils

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/brynbellomy/go-utils/errors"
)

// UnionTag is the struct tag key used for union field matching
const UnionTag = "union"

// UnmarshalUnion is a generic function to unmarshal tagged unions
func UnmarshalUnion(data []byte, v any) error {
	// Get the reflect.Value of the interface
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("v must be a non-nil pointer")
	}
	rv = rv.Elem()

	// Unmarshal into a map to get the discriminator field
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// Find the discriminator field and its value
	var discriminatorField reflect.Value
	var discriminatorJSONKey string
	var discriminatorValue any
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Type().Field(i)
		unionTag := field.Tag.Get(UnionTag)
		if unionTag == "@discriminator" {
			jsonTag := field.Tag.Get("json")
			if jsonTag != "" && jsonTag != "-" {
				discriminatorField = rv.Field(i)
				discriminatorJSONKey = strings.Split(jsonTag, ",")[0]
				discriminatorValue = m[discriminatorJSONKey]
				break
			}
		}
	}
	if !discriminatorField.IsValid() {
		return errors.New("@discriminator field not found")
	}

	// Find the matching union field
	var matchingField reflect.Value
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Type().Field(i)
		unionTag := field.Tag.Get(UnionTag)
		if unionTag == discriminatorValue {
			matchingField = rv.Field(i)
			break
		}
	}
	if !matchingField.IsValid() {
		return errors.Errorf("no matching union field found for %s=%v", discriminatorJSONKey, discriminatorValue)
	}

	// Create a new instance of the matching field's type
	newValue := reflect.New(matchingField.Type().Elem())

	// Unmarshal the data into the new instance
	if err := json.Unmarshal(data, newValue.Interface()); err != nil {
		return err
	}

	// Set the matching field to the new instance
	matchingField.Set(newValue)

	// Set the discriminator field value
	discriminatorField.Set(reflect.ValueOf(fmt.Sprint(discriminatorValue)))

	return nil
}

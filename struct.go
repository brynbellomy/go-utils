package utils

import (
	"fmt"
	"reflect"
)

func StructFieldNames(myStruct any) []string {
	val := reflect.ValueOf(myStruct)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	typ := val.Type()
	var fieldNames []string

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath == "" { // PkgPath is empty for exported fields
			fieldNames = append(fieldNames, field.Name)
		}
	}

	return fieldNames
}

func StructFieldValueStrings(myStruct any) []string {
	val := reflect.ValueOf(myStruct)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	var fieldValues []string
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)

		var strValue string
		// switch field.Kind() {
		// case reflect.String:
		//  strValue = field.String()
		// case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		//  strValue = strconv.FormatInt(field.Int(), 10)
		// case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		//  strValue = strconv.FormatUint(field.Uint(), 10)
		// case reflect.Float32, reflect.Float64:
		//  strValue = strconv.FormatFloat(field.Float(), 'f', -1, 64)
		// case reflect.Bool:
		//  strValue = strconv.FormatBool(field.Bool())
		// default:
		strValue = fmt.Sprintf("%v", field.Interface())
		// }
		fieldValues = append(fieldValues, strValue)
	}
	return fieldValues
}

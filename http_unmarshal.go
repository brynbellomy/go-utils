package utils

import (
	"context"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/brynbellomy/go-utils/errors"
)

type contextExtractorFn = func(ctx context.Context, key string) any

var contextValueExtractor contextExtractorFn = func(ctx context.Context, key string) any {
	return ctx.Value(key)
}

// SetContextExtractor configures a custom function for extracting values from request contexts.
// This is used when unmarshaling fields tagged with ctx:"key". The default implementation
// uses context.Context.Value(key), but frameworks may need custom extraction logic.
func SetContextExtractor(fn contextExtractorFn) {
	contextValueExtractor = fn
}

type paramExtractorFn = func(r *http.Request, param string) string

var paramValueExtractor paramExtractorFn

// SetParamExtractor configures a custom function for extracting URL path parameters.
// This is used when unmarshaling fields tagged with param:"name". Different routing
// frameworks (e.g., chi, gorilla/mux, gin) have different APIs for accessing path parameters,
// so this function must be set to match your router's API.
func SetParamExtractor(fn paramExtractorFn) {
	paramValueExtractor = fn
}

var unmarshalRequestRegexp = regexp.MustCompile(`(header|query|path|param|ctx|form|file|body):"([^"]*)"`)

// MultipartFile wraps a multipart file upload with its associated metadata header.
// It provides access to both the file contents and metadata such as filename, size,
// and MIME type through the Header field.
type MultipartFile struct {
	File   multipart.File
	Header *multipart.FileHeader
}

// Close closes the underlying file, releasing any associated resources.
func (mf *MultipartFile) Close() error {
	if mf.File != nil {
		return mf.File.Close()
	}
	return nil
}

// ensureFormParsed ensures that the request form/multipart form has been parsed
func ensureFormParsed(r *http.Request) error {
	if r.Form != nil {
		return nil // Already parsed
	}

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Parse multipart form with 64MB max memory
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			return errors.Errorf("failed to parse multipart form: %w", err)
		}
	} else if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		// Parse regular form
		if err := r.ParseForm(); err != nil {
			return errors.Errorf("failed to parse form: %w", err)
		}
	}
	return nil
}

// UnmarshalHTTPRequest extracts data from an HTTP request into a struct using struct tags.
// The into parameter must be a pointer to a struct. Supported tags include:
//
//   - header:"Header-Name" - extracts from request headers
//   - query:"param" - extracts from URL query parameters
//   - path:"pattern" - extracts the URL path
//   - param:"name" - extracts URL path parameters (requires SetParamExtractor)
//   - ctx:"key" - extracts from request context (requires SetContextExtractor)
//   - form:"field" - extracts from form data (application/x-www-form-urlencoded or multipart/form-data)
//   - file:"field" - extracts file uploads as *MultipartFile or []*MultipartFile
//   - body:"json" - unmarshals request body as JSON
//   - required:"true" - makes the field required (returns error if missing)
//
// Fields can be strings, integers, booleans, slices, or types implementing custom unmarshalers.
//
// Example usage:
//
//	type CreateUserRequest struct {
//	    UserID      string   `param:"id" required:"true"`
//	    Name        string   `form:"name" required:"true"`
//	    Email       string   `form:"email"`
//	    Age         int      `form:"age"`
//	    Tags        []string `query:"tag"`
//	    ContentType string   `header:"Content-Type"`
//	    Avatar      *MultipartFile `file:"avatar"`
//	}
//
//	func HandleCreateUser(w http.ResponseWriter, r *http.Request) {
//	    var req CreateUserRequest
//	    if err := UnmarshalHTTPRequest(&req, r); err != nil {
//	        http.Error(w, err.Error(), http.StatusBadRequest)
//	        return
//	    }
//	    // Use req.UserID, req.Name, etc.
//	    if req.Avatar != nil {
//	        defer req.Avatar.Close()
//	        // Process uploaded file...
//	    }
//	}
//
// For JSON body unmarshaling:
//
//	type UpdateSettingsRequest struct {
//	    Settings map[string]any `body:"json"`
//	}
//
// For multiple file uploads:
//
//	type UploadRequest struct {
//	    Images []*MultipartFile `file:"images"`
//	}
//
// For slice query parameters (?tag=foo&tag=bar):
//
//	type SearchRequest struct {
//	    Tags []string `query:"tag"`
//	}
func UnmarshalHTTPRequest(into any, r *http.Request) error {
	rval := reflect.ValueOf(into).Elem()

	// Check if we need to parse forms (scan for form: or file: tags)
	needsFormParsing := false
	for i := 0; i < rval.Type().NumField(); i++ {
		field := rval.Type().Field(i)
		if strings.Contains(string(field.Tag), `form:"`) || strings.Contains(string(field.Tag), `file:"`) {
			needsFormParsing = true
			break
		}
	}
	if needsFormParsing {
		if err := ensureFormParsed(r); err != nil {
			return err
		}
	}

	for i := 0; i < rval.Type().NumField(); i++ {
		field := rval.Type().Field(i)
		fieldVal := rval.Field(i)
		if !fieldVal.CanAddr() {
			return errors.Errorf("cannot unmarshal into unaddressable struct field '%v'", field.Name)
		}
		fieldVal = fieldVal.Addr()

		var found bool
		var value string
		var values []string
		var unmarshal func(fieldName, value string, values []string, fieldVal reflect.Value) error

		matches := unmarshalRequestRegexp.FindAllStringSubmatch(string(field.Tag), -1)
		if len(matches) == 0 {
			continue
		}

		source := matches[0][1]
		var arg string
		if len(matches[0]) > 2 {
			arg = matches[0][2]
		}

		switch source {
		case "method":
			value = r.Method
			unmarshal = unmarshalHTTPMethod
			found = true
		case "header":
			value = r.Header.Get(arg)
			unmarshal = unmarshalHTTPHeader
			found = len(value) > 0
		case "query":
			if r.URL.Query().Has(arg) {
				if fieldVal.Elem().Kind() == reflect.Slice {
					values = r.URL.Query()[arg]
					unmarshal = unmarshalURLQuery
				} else {
					value = r.URL.Query().Get(arg)
					values = r.URL.Query()[arg]
					unmarshal = unmarshalURLQuery
				}
				found = true
			}

		case "path":
			value = r.URL.Path
			unmarshal = unmarshalURLPath
			found = true

		case "body":
			bs, err := io.ReadAll(r.Body)
			if err != nil {
				return err
			}
			value = string(bs)
			if arg == "json" {
				unmarshal = unmarshalBodyJSON
			} else {
				return errors.Errorf("unsupported body format '%s'", arg)
			}
			found = len(bs) > 0

		case "param":
			if paramValueExtractor == nil {
				return errors.Errorf("no param extractor registered")
			}
			value = paramValueExtractor(r, arg)
			unmarshal = unmarshalRouteParam
			found = len(value) > 0

		case "ctx":
			if contextValueExtractor == nil {
				return errors.Errorf("no context extractor registered")
			}
			ctxValue := contextValueExtractor(r.Context(), arg)
			if ctxValue == nil {
				break
			}
			rctxValue := reflect.ValueOf(ctxValue)
			if !rctxValue.IsValid() {
				break
			}
			targetType := fieldVal.Type().Elem()

			// Treat nil-able kinds specially to avoid panics
			switch rctxValue.Kind() {
			case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
				if rctxValue.IsNil() {
					break
				}
			}

			if !rctxValue.Type().AssignableTo(targetType) {
				return errors.Errorf("cannot assign context value of type %v to field '%s' of type %v", rctxValue.Type(), field.Name, targetType)
			}
			fieldVal.Elem().Set(rctxValue)
			found = true

		case "form":
			// Extract form field value
			if r.Form.Has(arg) {
				if fieldVal.Elem().Kind() == reflect.Slice {
					values = r.Form[arg]
					unmarshal = unmarshalFormField
				} else {
					value = r.Form.Get(arg)
					unmarshal = unmarshalFormField
				}
				found = true
			}

		case "file":
			// Extract file upload(s)
			// Check if this is a multiple file field
			if fieldVal.Elem().Kind() == reflect.Slice {
				// Multiple files: []*MultipartFile
				files := r.MultipartForm.File[arg]
				if len(files) == 0 {
					break // No files uploaded
				}

				// Create slice of MultipartFile pointers
				slice := reflect.MakeSlice(fieldVal.Type().Elem(), 0, len(files))
				for _, fileHeader := range files {
					file, err := fileHeader.Open()
					if err != nil {
						return errors.Errorf("failed to open file '%s': %w", arg, err)
					}
					mf := &MultipartFile{
						File:   file,
						Header: fileHeader,
					}
					slice = reflect.Append(slice, reflect.ValueOf(mf))
				}
				fieldVal.Elem().Set(slice)
				found = true
				break
			} else {
				// Single file: *MultipartFile
				file, fileHeader, err := r.FormFile(arg)
				if err == http.ErrMissingFile {
					break // File not provided
				} else if err != nil {
					return errors.Errorf("failed to get file '%s': %w", arg, err)
				}
				mf := &MultipartFile{
					File:   file,
					Header: fileHeader,
				}
				fieldVal.Elem().Set(reflect.ValueOf(mf))
				found = true
				break
			}
		}

		if !found && field.Tag.Get("required") == "true" {
			return errors.Errorf("missing request field '%v'", field.Name)
		}

		if unmarshal != nil {
			err := unmarshal(arg, value, values, fieldVal)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func unmarshalBodyJSON(fieldName, value string, values []string, fieldVal reflect.Value) error {
	// fieldVal is already an address (pointer) from the caller
	// We need to pass the pointer interface to json.Unmarshal
	ptr := fieldVal.Interface()

	err := json.Unmarshal([]byte(value), ptr)
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal JSON body into field '%s'", fieldName)
	}
	return nil
}

func unmarshalHTTPMethod(fieldName, method string, _ []string, fieldVal reflect.Value) error {
	return unmarshalHTTPField(fieldName, method, nil, fieldVal)
}

// URLPathUnmarshaler is implemented by types that can unmarshal themselves from a URL path string.
// When a struct field implements this interface and is tagged with path:"", this method will be
// called instead of the default string conversion.
type URLPathUnmarshaler interface {
	UnmarshalURLPath(path string) error
}

func unmarshalURLPath(fieldName, path string, _ []string, fieldVal reflect.Value) error {
	val := fieldVal.Interface()
	if as, is := val.(URLPathUnmarshaler); is {
		return as.UnmarshalURLPath(path)
	}
	return unmarshalHTTPField(fieldName, path, nil, fieldVal)
}

// URLQueryUnmarshaler is implemented by types that can unmarshal themselves from URL query parameter values.
// When a struct field implements this interface and is tagged with query:"param", this method will be
// called with all values for that query parameter instead of the default conversion.
type URLQueryUnmarshaler interface {
	UnmarshalURLQuery(values []string) error
}

func unmarshalURLQuery(fieldName, value string, values []string, fieldVal reflect.Value) error {
	val := fieldVal.Interface()
	if as, is := val.(URLQueryUnmarshaler); is {
		return as.UnmarshalURLQuery(values)
	}
	return unmarshalHTTPField(fieldName, value, values, fieldVal)
}

// HTTPHeaderUnmarshaler is implemented by types that can unmarshal themselves from an HTTP header value.
// When a struct field implements this interface and is tagged with header:"Header-Name", this method
// will be called instead of the default string conversion.
type HTTPHeaderUnmarshaler interface {
	UnmarshalHTTPHeader(header string) error
}

func unmarshalHTTPHeader(fieldName, header string, _ []string, fieldVal reflect.Value) error {
	val := fieldVal.Interface()
	if as, is := val.(HTTPHeaderUnmarshaler); is {
		return as.UnmarshalHTTPHeader(header)
	}
	return unmarshalHTTPField(fieldName, header, nil, fieldVal)
}

// RouteParamUnmarshaler is implemented by types that can unmarshal themselves from a URL route parameter.
// When a struct field implements this interface and is tagged with param:"name", this method will be
// called instead of the default string conversion.
type RouteParamUnmarshaler interface {
	UnmarshalRouteParam(param string) error
}

func unmarshalRouteParam(fieldName, param string, _ []string, fieldVal reflect.Value) error {
	val := fieldVal.Interface()
	if as, is := val.(RouteParamUnmarshaler); is {
		return as.UnmarshalRouteParam(param)
	}
	return unmarshalHTTPField(fieldName, param, nil, fieldVal)
}

// FormFieldUnmarshaler is implemented by types that can unmarshal themselves from a form field value.
// When a struct field implements this interface and is tagged with form:"field", this method will be
// called instead of the default string conversion.
type FormFieldUnmarshaler interface {
	UnmarshalFormField(value string) error
}

func unmarshalFormField(fieldName, value string, values []string, fieldVal reflect.Value) error {
	val := fieldVal.Interface()
	if as, is := val.(FormFieldUnmarshaler); is {
		return as.UnmarshalFormField(value)
	}
	return unmarshalHTTPField(fieldName, value, values, fieldVal)
}

func unmarshalHTTPField(fieldName, value string, values []string, fieldVal reflect.Value) error {
	if as, is := fieldVal.Interface().(encoding.TextUnmarshaler); is {
		return as.UnmarshalText([]byte(value))
	}

	// Handle string wrapper types
	rval := reflect.ValueOf(value)
	if rval.Type().ConvertibleTo(fieldVal.Type().Elem()) {
		fieldVal.Elem().Set(rval.Convert(fieldVal.Type().Elem()))
		return nil
	}

	switch fieldVal.Type().Elem().Kind() {
	case reflect.Pointer:
		v := reflect.New(fieldVal.Type().Elem().Elem())
		err := unmarshalHTTPField(fieldName, value, values, v)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(v)
		return nil

	case reflect.Slice:
		// Special case: []byte should be treated as a byte array, not a slice of values
		if fieldVal.Type().Elem() == reflect.TypeFor[[]byte]() {
			// Check if we have a single value or multiple values
			if len(values) > 0 {
				fieldVal.Elem().Set(reflect.ValueOf([]byte(values[0])))
			} else {
				fieldVal.Elem().Set(reflect.ValueOf([]byte(value)))
			}
			return nil
		}
		slice := reflect.MakeSlice(fieldVal.Type().Elem(), 0, len(values))
		sliceElemType := fieldVal.Type().Elem().Elem()

		for i, v := range values {
			elem := reflect.New(sliceElemType)
			err := unmarshalHTTPField(fieldName+fmt.Sprintf("[%v]", i), v, nil, elem)
			if err != nil {
				return err
			}
			slice = reflect.Append(slice, elem.Elem())
		}
		fieldVal.Elem().Set(slice)
		return nil

	case reflect.Int:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(reflect.ValueOf(int(n)).Convert(fieldVal.Type().Elem()))
	case reflect.Int8:
		n, err := strconv.ParseInt(value, 10, 8)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(reflect.ValueOf(int8(n)).Convert(fieldVal.Type().Elem()))
	case reflect.Int16:
		n, err := strconv.ParseInt(value, 10, 16)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(reflect.ValueOf(int16(n)).Convert(fieldVal.Type().Elem()))
	case reflect.Int32:
		n, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(reflect.ValueOf(int32(n)).Convert(fieldVal.Type().Elem()))
	case reflect.Int64:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(reflect.ValueOf(int64(n)).Convert(fieldVal.Type().Elem()))

	case reflect.Uint:
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(reflect.ValueOf(uint(n)).Convert(fieldVal.Type().Elem()))
	case reflect.Uint8:
		n, err := strconv.ParseUint(value, 10, 8)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(reflect.ValueOf(uint8(n)).Convert(fieldVal.Type().Elem()))
	case reflect.Uint16:
		n, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(reflect.ValueOf(uint16(n)).Convert(fieldVal.Type().Elem()))
	case reflect.Uint32:
		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(reflect.ValueOf(uint32(n)).Convert(fieldVal.Type().Elem()))
	case reflect.Uint64:
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(reflect.ValueOf(uint64(n)).Convert(fieldVal.Type().Elem()))

	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(reflect.ValueOf(b).Convert(fieldVal.Type().Elem()))

	default:
		panic(fmt.Sprintf(`cannot unmarshal http.Request field "%v" into type %v`, fieldName, fieldVal))
	}
	return nil
}

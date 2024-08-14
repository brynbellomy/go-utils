package utils

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/cors"

	"github.com/brynbellomy/go-utils/errors"
)

var dnsCache = NewSyncMap[string, string]()

func ApplyCachedDNS(urlStr string) (string, func(), error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", nil, err
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return "", nil, fmt.Errorf("No hostname in url: %s", parsedURL)
	}

	cachedIP, ok := dnsCache.Get(hostname)
	if !ok {
		ips, err := net.LookupIP(hostname)
		if err != nil {
			return "", nil, err
		}
		cachedIP = ips[0].String()
		dnsCache.Set(hostname, cachedIP)
	}

	resolvedURL := *parsedURL
	resolvedURL.Host = strings.Replace(parsedURL.Host, hostname, cachedIP, 1)
	return resolvedURL.String(), func() { dnsCache.Delete(hostname) }, nil
}

func JSONRequest(ctx context.Context, method string, url string, body any, headers http.Header, response any) (http.Header, int, error) {
	if headers == nil {
		headers = http.Header{}
	}

	headers["Accept"] = []string{"application/json"}
	headers["Content-Type"] = []string{"application/json"}

	var bs []byte
	var err error
	if body != nil && !reflect.ValueOf(body).IsZero() {
		bs, err = json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
	}

	resp, err := HTTPRequest(ctx, method, url, bytes.NewReader(bs), headers)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return resp.Header, resp.StatusCode, err
	}
	return resp.Header, resp.StatusCode, nil
}

var LogHTTPRequests bool

func HTTPRequest(ctx context.Context, method string, urlStr string, body io.Reader, headers http.Header) (*http.Response, error) {
	urlWithCachedDNS, clearDNSForHostname, err := ApplyCachedDNS(urlStr)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, urlWithCachedDNS, body)
	if err != nil {
		return nil, err
	}

	req.Header = headers
	if headers == nil {
		req.Header = http.Header{}
	}

	if LogHTTPRequests {
		reqDump, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			return nil, err
		}
		fmt.Println("REQUEST:", string(reqDump))
	}

	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		if _, ok := err.(*url.Error); ok {
			clearDNSForHostname()
		}
		return nil, err
	}

	if LogHTTPRequests {
		fmt.Println("STATUS:", resp.StatusCode, resp.Status)
	}
	return resp, nil
}

type HTTPClient struct {
	http.Client
	chStop chan struct{}
}

func MakeHTTPClient(requestTimeout, reapIdleConnsInterval time.Duration, cookieJar http.CookieJar, tlsCerts []tls.Certificate) *HTTPClient {
	c := http.Client{
		Timeout: requestTimeout,
		Jar:     cookieJar,
	}

	c.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS13,
			MaxVersion:         tls.VersionTLS13,
			Certificates:       tlsCerts,
			ClientAuth:         tls.RequestClientCert,
			InsecureSkipVerify: true,
		},
	}

	chStop := make(chan struct{})

	if reapIdleConnsInterval > 0 {
		go func() {
			ticker := time.NewTicker(reapIdleConnsInterval)
			defer ticker.Stop()
			defer c.CloseIdleConnections()

			for {
				select {
				case <-ticker.C:
					c.CloseIdleConnections()
				case <-chStop:
					return
				}
			}
		}()
	}

	return &HTTPClient{c, chStop}
}

func (c HTTPClient) Close() {
	close(c.chStop)
}

var unmarshalRequestRegexp = regexp.MustCompile(`(header|query|path):"([^"]*)"`)
var stringType = reflect.TypeOf("")

func UnmarshalHTTPRequest(into any, r *http.Request) error {
	rval := reflect.ValueOf(into).Elem()

	for i := 0; i < rval.Type().NumField(); i++ {
		field := rval.Type().Field(i)
		matches := unmarshalRequestRegexp.FindAllStringSubmatch(string(field.Tag), -1)
		var found bool
		for _, match := range matches {
			source := match[1]
			var name string
			if len(match) > 2 {
				name = match[2]
			}

			fieldVal := rval.Field(i)
			if !fieldVal.CanAddr() {
				return errors.Errorf("cannot unmarshal into unaddressable struct field '%v'", field.Name)
			}
			fieldVal = fieldVal.Addr()

			var value string
			var values []string
			var unmarshal func(fieldName, value string, values []string, fieldVal reflect.Value) error
			switch source {
			case "method":
				value = r.Method
				unmarshal = unmarshalHTTPMethod
			case "header":
				value = r.Header.Get(name)
				unmarshal = unmarshalHTTPHeader
			case "query":
				if r.URL.Query().Has(name) {
					if fieldVal.Elem().Kind() == reflect.Slice {
						values = r.URL.Query()[name]
						unmarshal = unmarshalURLQuery
					} else {
						value = r.URL.Query().Get(name)
						unmarshal = unmarshalURLQuery
					}
				}
			case "path":
				// if name == "" {
				value = r.URL.Path
				// }
				// else {
				//     idx, err := strconv.Atoi(name)
				//     if err != nil {
				//         return err
				//     }
				//     parts := strings.Split(r.URL.Path, "/")
				//     if idx >= len(parts) {
				//         panic("invariant violation")
				//     }
				// }
				unmarshal = unmarshalURLPath
			case "body":
				bs, err := ioutil.ReadAll(r.Body)
				if err != nil {
					return err
				}
				value = string(bs)
				unmarshal = unmarshalBody
			default:
				panic("invariant violation")
			}
			if value == "" && values == nil {
				continue
			}

			err := unmarshal(name, value, values, fieldVal)
			if err != nil {
				return err
			}
			found = true
			break
		}
		if !found {
			if field.Tag.Get("required") == "true" {
				return errors.Errorf("missing request field '%v'", field.Name)
			}
		}
	}
	return nil
}

func unmarshalBody(fieldName, value string, values []string, fieldVal reflect.Value) error {
	return json.Unmarshal([]byte(value), fieldVal.Interface())
}

var unmarshalResponseRegexp = regexp.MustCompile(`(header):"([^"]*)"`)

func UnmarshalHTTPResponse(into any, r *http.Response) error {
	rval := reflect.ValueOf(into).Elem()

	for i := 0; i < rval.Type().NumField(); i++ {
		field := rval.Type().Field(i)
		matches := unmarshalRequestRegexp.FindAllStringSubmatch(string(field.Tag), -1)
		var found bool
		for _, match := range matches {
			source := match[1]
			name := match[2]

			fieldVal := rval.Field(i)
			if fieldVal.Kind() == reflect.Ptr {
				// no-op
			} else if fieldVal.CanAddr() {
				fieldVal = fieldVal.Addr()
			} else {
				return errors.Errorf("cannot unmarshal into unaddressable struct field '%v'", field.Name)
			}

			var value string
			var unmarshal func(fieldName, value string, values []string, fieldVal reflect.Value) error
			switch source {
			case "header":
				value = r.Header.Get(name)
				unmarshal = unmarshalHTTPHeader
			default:
				panic("invariant violation")
			}
			if value == "" {
				continue
			}

			err := unmarshal(name, value, nil, fieldVal)
			if err != nil {
				return err
			}
			found = true
			break
		}
		if !found {
			if field.Tag.Get("required") != "" {
				return errors.Errorf("missing request field '%v'", field.Name)
			}
		}
	}
	return nil
}

func unmarshalHTTPMethod(fieldName, method string, _ []string, fieldVal reflect.Value) error {
	return unmarshalHTTPField(fieldName, method, nil, fieldVal)
}

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
	case reflect.Ptr:
		v := reflect.New(fieldVal.Type().Elem().Elem())
		err := unmarshalHTTPField(fieldName, value, values, v)
		if err != nil {
			return err
		}
		fieldVal.Elem().Set(v)
		return nil

	case reflect.Slice:
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

type MultipartPart struct {
	Part *multipart.Part
	Body io.ReadCloser
}

func (mp *MultipartPart) Read(p []byte) (n int, err error) {
	return mp.Part.Read(p)
}

func (mp *MultipartPart) Close() error {
	var err1, err2 error
	if mp.Part != nil {
		err1 = mp.Part.Close()
	}
	if mp.Body != nil {
		err2 = mp.Body.Close()
	}
	if err1 != nil {
		return err1
	}
	return err2
}

func ParseMultipartForm(header http.Header, body io.Reader, fn func(field string, part *multipart.Part) error) error {
	contentTypeHeader := header.Get("Content-Type")
	_, params, err := mime.ParseMediaType(contentTypeHeader)
	if err != nil {
		return err
	}
	boundary := params["boundary"]

	mr := multipart.NewReader(body, boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		defer part.Close()

		err = fn(part.FormName(), part)
		if err != nil {
			return err
		}
	}
	return nil
}

func RespondJSON(resp http.ResponseWriter, data interface{}) {
	resp.Header().Add("Content-Type", "application/json")

	err := json.NewEncoder(resp).Encode(data)
	if err != nil {
		panic(err)
	}
}

func UnrestrictedCors(handler http.Handler) http.Handler {
	return cors.New(cors.Options{
		AllowOriginFunc:  func(string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "AUTHORIZE", "SUBSCRIBE", "ACK", "OPTIONS", "HEAD"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: true,
	}).Handler(handler)
}

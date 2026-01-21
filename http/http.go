package bhttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/brynbellomy/go-utils/coll"
	"github.com/rs/cors"
)

// dnsCache is a thread-safe map that stores hostname to IP address mappings for DNS caching.
var dnsCache = bcoll.NewSyncMap[string, string]()

// ApplyCachedDNS resolves the hostname in the given URL using a cached DNS lookup and returns
// a modified URL with the IP address substituted for the hostname. It also returns a cleanup
// function that removes the cached entry for the hostname.
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

// JSONRequest performs an HTTP request with JSON encoding/decoding. It marshals the body
// parameter to JSON, sends the request with appropriate Content-Type and Accept headers,
// and unmarshals the response into the response parameter. It returns the response headers,
// status code, and any error encountered.
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

// LogHTTPRequests controls whether HTTP requests and responses are logged to stdout.
// When set to true, request dumps and response status codes will be printed.
var LogHTTPRequests bool

// HTTPRequest performs an HTTP request with the given method, URL, body, and headers.
// It uses cached DNS resolution and will clear the DNS cache for the hostname if a URL error occurs.
// If LogHTTPRequests is true, it will log the request and response details to stdout.
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

// HTTPClient extends http.Client with automatic idle connection reaping capabilities.
// It includes a stop channel for graceful shutdown of the connection reaping goroutine.
type HTTPClient struct {
	http.Client
	chStop chan struct{}
}

// MakeHTTPClient creates a new HTTPClient with the specified configuration. The requestTimeout
// sets the maximum duration for requests. If reapIdleConnsInterval is greater than 0, a goroutine
// will periodically close idle connections at the specified interval. The client uses TLS 1.3 only
// and accepts the provided TLS certificates for client authentication.
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

// Close stops the idle connection reaping goroutine by closing the stop channel.
func (c HTTPClient) Close() {
	close(c.chStop)
}

// MultipartPart wraps a multipart.Part and its associated body reader, implementing
// io.ReadCloser. It ensures both the part and body are properly closed.
type MultipartPart struct {
	Part *multipart.Part
	Body io.ReadCloser
}

// Read implements io.Reader by delegating to the underlying Part's Read method.
func (mp *MultipartPart) Read(p []byte) (n int, err error) {
	return mp.Part.Read(p)
}

// Close implements io.Closer by closing both the Part and Body, returning the first error
// encountered if any.
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

// ParseMultipartForm parses a multipart form from the given header and body, invoking the
// provided callback function for each part. The callback receives the form field name and
// the part itself. Parsing stops on the first error returned by the callback.
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

// RespondJSON encodes the given data as JSON and writes it to the response writer with
// the appropriate Content-Type header. It panics if encoding fails.
func RespondJSON(resp http.ResponseWriter, data any) {
	resp.Header().Add("Content-Type", "application/json")

	err := json.NewEncoder(resp).Encode(data)
	if err != nil {
		panic(err)
	}
}

// UnrestrictedCORS wraps an HTTP handler with permissive CORS middleware that allows
// all origins, methods, headers, and credentials. This should only be used in development
// or when the API is intentionally public.
func UnrestrictedCORS(handler http.Handler) http.Handler {
	return cors.New(cors.Options{
		AllowOriginFunc:  func(string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "AUTHORIZE", "SUBSCRIBE", "ACK", "OPTIONS", "HEAD"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: true,
	}).Handler(handler)
}

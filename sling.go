package sling

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	goquery "github.com/google/go-querystring/query"
)

const (
	contentType     = "Content-Type"
	jsonContentType = "application/json"
	formContentType = "application/x-www-form-urlencoded"
)

// Doer executes http requests.  It is implemented by *http.Client.  You can
// wrap *http.Client with layers of Doers to form a stack of client-side
// middleware.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Sling is an HTTP Request builder and sender.
type Sling struct {
	// http Client for doing requests
	httpClient Doer
	// HTTP method (GET, POST, etc.)
	method string
	// parsed url string for requests
	baseURL *url.URL
	// parsed url string for requests
	pathURL *url.URL
	// stores key-values pairs to add to request's Headers
	header http.Header
	// url tagged query structs
	queryStructs []interface{}
	// json tagged body struct
	bodyJSON interface{}
	// url tagged body struct (form)
	bodyForm interface{}
	// simply assigned body
	body io.ReadCloser
}

// New returns a new Sling with an http DefaultClient.
func New() *Sling {
	return &Sling{
		httpClient:   http.DefaultClient,
		method:       "GET",
		header:       make(http.Header),
		queryStructs: make([]interface{}, 0),
	}
}

// New returns a copy of a Sling for creating a new Sling with properties
// from a parent Sling. For example,
//
// 	parentSling := sling.New().Client(client).Base("https://api.io/")
// 	fooSling := parentSling.New().Get("foo/")
// 	barSling := parentSling.New().Get("bar/")
//
// fooSling and barSling will both use the same client, but send requests to
// https://api.io/foo/ and https://api.io/bar/ respectively.
//
// Note that query and body values are copied so if pointer values are used,
// mutating the original value will mutate the value within the child Sling.
func (s *Sling) New() *Sling {
	// copy Headers pairs into new Header map
	headerCopy := make(http.Header)
	for k, v := range s.header {
		headerCopy[k] = v
	}
	return &Sling{
		httpClient:   s.httpClient,
		method:       s.method,
		baseURL:      s.baseURL,
		pathURL:      s.pathURL,
		header:       headerCopy,
		queryStructs: append([]interface{}{}, s.queryStructs...),
		bodyJSON:     s.bodyJSON,
		bodyForm:     s.bodyForm,
		body:         s.body,
	}
}

// Http Client

// Client sets the http Client used to do requests. If a nil client is given,
// the http.DefaultClient will be used.
func (s *Sling) Client(httpClient *http.Client) *Sling {
	if httpClient == nil {
		return s.Doer(http.DefaultClient)
	}
	return s.Doer(httpClient)
}

// Doer sets the custom Doer implementation used to do requests.
// If a nil client is given, the http.DefaultClient will be used.
func (s *Sling) Doer(doer Doer) *Sling {
	if doer == nil {
		s.httpClient = http.DefaultClient
	} else {
		s.httpClient = doer
	}
	return s
}

// Method

// Head sets the Sling method to HEAD and sets the given pathURL.
func (s *Sling) Head(pathURL string) *Sling {
	s.method = "HEAD"
	return s.Path(pathURL)
}

// Get sets the Sling method to GET and sets the given pathURL.
func (s *Sling) Get(pathURL string) *Sling {
	s.method = "GET"
	return s.Path(pathURL)
}

// Post sets the Sling method to POST and sets the given pathURL.
func (s *Sling) Post(pathURL string) *Sling {
	s.method = "POST"
	return s.Path(pathURL)
}

// Put sets the Sling method to PUT and sets the given pathURL.
func (s *Sling) Put(pathURL string) *Sling {
	s.method = "PUT"
	return s.Path(pathURL)
}

// Patch sets the Sling method to PATCH and sets the given pathURL.
func (s *Sling) Patch(pathURL string) *Sling {
	s.method = "PATCH"
	return s.Path(pathURL)
}

// Delete sets the Sling method to DELETE and sets the given pathURL.
func (s *Sling) Delete(pathURL string) *Sling {
	s.method = "DELETE"
	return s.Path(pathURL)
}

// Header

// Add adds the key, value pair in Headers, appending values for existing keys
// to the key's values. Header keys are canonicalized.
func (s *Sling) Add(key, value string) *Sling {
	s.header.Add(key, value)
	return s
}

// Set sets the key, value pair in Headers, replacing existing values
// associated with key. Header keys are canonicalized.
func (s *Sling) Set(key, value string) *Sling {
	s.header.Set(key, value)
	return s
}

// SetBasicAuth sets the Authorization header to use HTTP Basic Authentication
// with the provided username and password. With HTTP Basic Authentication
// the provided username and password are not encrypted.
func (s *Sling) SetBasicAuth(username, password string) *Sling {
	return s.Set("Authorization", "Basic "+basicAuth(username, password))
}

// basicAuth returns the base64 encoded username:password for basic auth copied
// from net/http.
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// Url

// Base sets the baseURL. If you intend to extend the url with Path,
// baseUrl should be specified with a trailing slash.
func (s *Sling) Base(baseURL string) *Sling {
	s.baseURL, _ = url.Parse(baseURL)
	return s
}

// Path extends the baseURL with the given path by resolving the reference to
// an absolute URL.
func (s *Sling) Path(path string) *Sling {
	pathURL, _ := url.Parse(path)
	if s.pathURL != nil {
		s.pathURL = s.pathURL.ResolveReference(pathURL)
	} else {
		s.pathURL = pathURL
	}
	return s
}

// QueryStruct appends the queryStruct to the Sling's queryStructs. The value
// pointed to by each queryStruct will be encoded as url query parameters on
// new requests (see Request()).
// The queryStruct argument should be a pointer to a url tagged struct. See
// https://godoc.org/github.com/google/go-querystring/query for details.
func (s *Sling) QueryStruct(queryStruct interface{}) *Sling {
	if queryStruct != nil {
		s.queryStructs = append(s.queryStructs, queryStruct)
	}
	return s
}

// Body

// BodyJSON sets the Sling's bodyJSON. The value pointed to by the bodyJSON
// will be JSON encoded as the Body on new requests (see Request()).
// The bodyJSON argument should be a pointer to a JSON tagged struct. See
// https://golang.org/pkg/encoding/json/#MarshalIndent for details.
func (s *Sling) BodyJSON(bodyJSON interface{}) *Sling {
	if bodyJSON != nil {
		s.bodyJSON = bodyJSON
		s.Set(contentType, jsonContentType)
	}
	return s
}

// BodyForm sets the Sling's bodyForm. The value pointed to by the bodyForm
// will be url encoded as the Body on new requests (see Request()).
// The bodyStruct argument should be a pointer to a url tagged struct. See
// https://godoc.org/github.com/google/go-querystring/query for details.
func (s *Sling) BodyForm(bodyForm interface{}) *Sling {
	if bodyForm != nil {
		s.bodyForm = bodyForm
		s.Set(contentType, formContentType)
	}
	return s
}

// Body sets the Sling's body. The body value will be set as the Body on new
// requests (see Request()).
// If the provided body is also an io.Closer, the request Body will be closed
// by http.Client methods.
func (s *Sling) Body(body io.Reader) *Sling {
	rc, ok := body.(io.ReadCloser)
	if !ok && body != nil {
		rc = ioutil.NopCloser(body)
	}
	if rc != nil {
		s.body = rc
	}
	return s
}

func (s *Sling) GetURL() string {
	if s.baseURL != nil {
		if s.pathURL != nil {
			return s.baseURL.ResolveReference(s.pathURL).String()
		}
		return s.baseURL.String()
	} else if s.pathURL != nil {
		return s.pathURL.String()
	}
	return ""
}

// Requests

// Request returns a new http.Request created with the Sling properties.
// Returns any errors parsing the rawURL, encoding query structs, encoding
// the body, or creating the http.Request.
func (s *Sling) Request() (*http.Request, error) {
	reqURL, err := url.Parse(s.GetURL())
	if err != nil {
		return nil, err
	}
	err = addQueryStructs(reqURL, s.queryStructs)
	if err != nil {
		return nil, err
	}
	body, err := s.getRequestBody()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(s.method, reqURL.String(), body)
	if err != nil {
		return nil, err
	}
	addHeaders(req, s.header)
	return req, err
}

// addQueryStructs parses url tagged query structs using go-querystring to
// encode them to url.Values and format them onto the url.RawQuery. Any
// query parsing or encoding errors are returned.
func addQueryStructs(reqURL *url.URL, queryStructs []interface{}) error {
	urlValues, err := url.ParseQuery(reqURL.RawQuery)
	if err != nil {
		return err
	}
	// encodes query structs into a url.Values map and merges maps
	for _, queryStruct := range queryStructs {
		queryValues, err := goquery.Values(queryStruct)
		if err != nil {
			return err
		}
		for key, values := range queryValues {
			for _, value := range values {
				urlValues.Add(key, value)
			}
		}
	}
	// url.Values format to a sorted "url encoded" string, e.g. "key=val&foo=bar"
	reqURL.RawQuery = urlValues.Encode()
	return nil
}

// getRequestBody returns the io.Reader which should be used as the body
// of new Requests.
func (s *Sling) getRequestBody() (body io.Reader, err error) {
	if s.bodyJSON != nil && s.header.Get(contentType) == jsonContentType {
		body, err = encodeBodyJSON(s.bodyJSON)
		if err != nil {
			return nil, err
		}
	} else if s.bodyForm != nil && s.header.Get(contentType) == formContentType {
		body, err = encodeBodyForm(s.bodyForm)
		if err != nil {
			return nil, err
		}
	} else if s.body != nil {
		body = s.body
	}
	return body, nil
}

// encodeBodyJSON JSON encodes the value pointed to by bodyJSON into an
// io.Reader, typically for use as a Request Body.
func encodeBodyJSON(bodyJSON interface{}) (io.Reader, error) {
	var buf = new(bytes.Buffer)
	if bodyJSON != nil {
		buf = &bytes.Buffer{}
		err := json.NewEncoder(buf).Encode(bodyJSON)
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

// encodeBodyForm url encodes the value pointed to by bodyForm into an
// io.Reader, typically for use as a Request Body.
func encodeBodyForm(bodyForm interface{}) (io.Reader, error) {
	values, err := goquery.Values(bodyForm)
	if err != nil {
		return nil, err
	}
	return strings.NewReader(values.Encode()), nil
}

// addHeaders adds the key, value pairs from the given http.Header to the
// request. Values for existing keys are appended to the keys values.
func addHeaders(req *http.Request, header http.Header) {
	for key, values := range header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
}

// Sending

// Receive creates a new HTTP request and returns the response.
// Any error creating the request, sending it, or decoding the response is returned.
// Receive is shorthand for calling Request and Do.
func (s *Sling) Receive() (response []byte, httpResponse *http.Response, err error) {
	req, err := s.Request()
	if err != nil {
		return nil, nil, err
	}
	return s.Do(req)
}

// Do sends an HTTP request and returns the response.
// Any error sending the request or decoding the response is returned.
func (s *Sling) Do(req *http.Request) (response []byte, httpResponse *http.Response, err error) {
	if httpResponse, err = s.httpClient.Do(req); err != nil {
		response = nil
		return
	}
	// httpResponse contains a non-nil resp.Body which must be closed
	defer httpResponse.Body.Close()
	if response, err = ioutil.ReadAll(httpResponse.Body); err != nil {
		response = nil
		return
	}
	return
}

package flickr

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	endpoint        = "https://api.flickr.com/services/rest/?"
	uploadEndpoint  = "https://api.flickr.com/services/upload/"
	replaceEndpoint = "https://api.flickr.com/services/replace/"
	oauthEndPoint   = "https://www.flickr.com/services/oauth/"
	authorizeUrl    = "https://www.flickr.com/services/oauth/authorize"
	apiHost         = "api.flickr.com"
	nonceBytes      = 32
)

type Request struct {
	ApiKey string
	Method string
	Args   map[string]string
	OAuth  *OAuth
}

type OAuth struct {
	ConsumerSecret   string
	Callback         string
	OAuthToken       string
	OAuthTokenSecret string
}

type Response struct {
	Status  string         `xml:"stat,attr"`
	Error   *ResponseError `xml:"err"`
	Payload string         `xml:",innerxml"`
}

type ResponseError struct {
	Code    string `xml:"code,attr"`
	Message string `xml:"msg,attr"`
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

type Error string

func (e Error) Error() string {
	return string(e)
}

func (request *Request) Sign(secret string) {
	args := request.Args

	// Remove api_sig
	delete(args, "api_sig")

	sorted_keys := make([]string, len(args)+2)

	args["api_key"] = request.ApiKey
	args["method"] = request.Method

	// Sort array keys
	i := 0
	for k := range args {
		sorted_keys[i] = k
		i++
	}
	sort.Strings(sorted_keys)

	// Build out ordered key-value string prefixed by secret
	s := secret
	for _, key := range sorted_keys {
		if args[key] != "" {
			s += fmt.Sprintf("%s%s", key, args[key])
		}
	}

	// Since we're only adding two keys, it's easier
	// and more space-efficient to just delete them
	// them copy the whole map
	delete(args, "api_key")
	delete(args, "method")

	// Have the full string, now hash
	hash := md5.New()
	hash.Write([]byte(s))

	// Add api_sig as one of the args
	args["api_sig"] = fmt.Sprintf("%x", hash.Sum(nil))
}

func (request *Request) URL() string {
	args := request.Args

	args["api_key"] = request.ApiKey
	args["method"] = request.Method

	s := endpoint + encodeQuery(args)
	return s
}

func (request *Request) Execute() (response string, ret error) {
	if request.ApiKey == "" || request.Method == "" {
		return "", Error("Need both API key and method")
	}

	s := request.URL()

	res, err := http.Get(s)
	defer res.Body.Close()
	if err != nil {
		return "", err
	}

	body, _ := ioutil.ReadAll(res.Body)
	return string(body), nil
}

func (request *Request) RequestToken() (map[string]string, error) {
	nonce, err := getNonce()
	if err != nil {
		return nil, err
	}
	// generating a signature requires ordered parameters, sorted by lexicographical
	// byte order
	args := make(map[string]string)
	args["oauth_nonce"] = nonce
	args["oauth_timestamp"] = strconv.Itoa(int(time.Now().Unix()))
	args["oauth_consumer_key"] = request.ApiKey
	args["oauth_signature_method"] = "HMAC-SHA1"
	args["oauth_version"] = "1.0"
	args["oauth_callback"] = request.OAuth.Callback
	key := request.OAuth.ConsumerSecret + "&"
	oauthSignature := request.oauthSignature(oauthEndPoint, args, "request_token", key)
	args["oauth_signature"] = oauthSignature
	s := oauthEndPoint + "request_token?" + encodeQuery(args)

	res, err := http.Get(s)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	kvs := strings.Split(string(body), "&")
	ret := make(map[string]string)
	for _, k := range kvs {
		kv := strings.SplitN(k, "=", 2)
		ret[kv[0]] = kv[1]
	}
	return ret, nil
}

func (request *Request) ExecuteAuthenticated() (string, error) {
	nonce, err := getNonce()
	if err != nil {
		return "", err
	}
	// generating a signature requires ordered parameters, sorted by lexicographical
	// byte order
	args := request.Args
	args["oauth_nonce"] = nonce
	args["oauth_timestamp"] = strconv.Itoa(int(time.Now().Unix()))
	args["oauth_consumer_key"] = request.ApiKey
	args["oauth_signature_method"] = "HMAC-SHA1"
	args["oauth_version"] = "1.0"
	args["method"] = request.Method
	args["oauth_token"] = request.OAuth.OAuthToken
	key := request.OAuth.ConsumerSecret + "&" + request.OAuth.OAuthTokenSecret
	oauthSignature := request.oauthSignature(endpoint, args, "", key)
	args["oauth_signature"] = oauthSignature
	s := endpoint + encodeQuery(args)

	res, err := http.Get(s)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var jsonBlob interface{}
	err = json.Unmarshal([]byte(string(body)), &jsonBlob)
	m := jsonBlob.(map[string]interface{})
	if m["stat"].(string) == "fail" {
		errStr := fmt.Sprintf("Response error: Code: %d; Message: %s", int(m["code"].(float64)), m["message"].(string))
		return "", errors.New(errStr)
	}

	return string(body), nil
}

func (request *Request) AuthorizeUrl(token map[string]string, perms string) string {
	return fmt.Sprintf("%s?oauth_token=%s&perms=%s", authorizeUrl, token["oauth_token"], perms)
}

func (request *Request) AccessToken(oauth_token string, oauth_verifier string, oauth_token_secret string) (map[string]string, error) {
	nonce, err := getNonce()
	if err != nil {
		return nil, err
	}
	// generating a signature requires ordered parameters, sorted by lexicographical
	// byte order
	args := make(map[string]string)
	args["oauth_nonce"] = nonce
	args["oauth_timestamp"] = strconv.Itoa(int(time.Now().Unix()))
	args["oauth_verifier"] = oauth_verifier
	args["oauth_consumer_key"] = request.ApiKey
	args["oauth_signature_method"] = "HMAC-SHA1"
	args["oauth_version"] = "1.0"
	args["oauth_token"] = oauth_token
	key := request.OAuth.ConsumerSecret + "&" + oauth_token_secret
	oauthSignature := request.oauthSignature(oauthEndPoint, args, "access_token", key)
	args["oauth_signature"] = oauthSignature
	s := oauthEndPoint + "access_token?" + encodeQuery(args)

	res, err := http.Get(s)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, _ := ioutil.ReadAll(res.Body)
	kvs := strings.Split(string(body), "&")
	ret := make(map[string]string)
	for _, k := range kvs {
		kv := strings.SplitN(k, "=", 2)
		ret[kv[0]], _ = url.QueryUnescape(kv[1])
	}
	return ret, nil
}

func (request *Request) oauthSignature(endpoint string, args map[string]string, method string, key string) string {
	sorted_keys := make([]string, len(args))

	// Sort array keys in order to compute signature
	i := 0
	for k := range args {
		sorted_keys[i] = k
		i++
	}
	sort.Strings(sorted_keys)

	baseString := ""
	for i, key := range sorted_keys {
		if i != 0 {
			baseString += "&"
		}
		baseString += fmt.Sprintf("%s=%s", key, url.QueryEscape(args[key]))
	}

	baseString = url.QueryEscape(baseString)
	encodedEndpoint := url.QueryEscape(strings.Replace(endpoint, "?", "", -1) + method)

	sigString := "GET&" + encodedEndpoint + "&" + baseString
	return computeSha1(sigString, key)
}

func computeSha1(message string, key string) string {
	hmac := hmac.New(sha1.New, []byte(key))
	hmac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(hmac.Sum(nil))
}

func getNonce() (string, error) {
	b := make([]byte, nonceBytes)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(b), nil
}

func encodeQuery(args map[string]string) string {
	i := 0
	s := bytes.NewBuffer(nil)
	for k, v := range args {
		if i != 0 {
			s.WriteString("&")
		}
		i++
		s.WriteString(k + "=" + url.QueryEscape(v))
	}
	return s.String()
}

func (request *Request) buildPost(url_ string, filename string, filetype string) (*http.Request, error) {
	real_url, _ := url.Parse(url_)

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	f_size := stat.Size()

	request.Args["api_key"] = request.ApiKey

	boundary, end := "----###---###--flickr-go-rules", "\r\n"

	// Build out all of POST body sans file
	header := bytes.NewBuffer(nil)
	for k, v := range request.Args {
		header.WriteString("--" + boundary + end)
		header.WriteString("Content-Disposition: form-data; name=\"" + k + "\"" + end + end)
		header.WriteString(v + end)
	}
	header.WriteString("--" + boundary + end)
	header.WriteString("Content-Disposition: form-data; name=\"photo\"; filename=\"photo.jpg\"" + end)
	header.WriteString("Content-Type: " + filetype + end + end)

	footer := bytes.NewBufferString(end + "--" + boundary + "--" + end)

	body_len := int64(header.Len()) + int64(footer.Len()) + f_size

	r, w := io.Pipe()
	go func() {
		pieces := []io.Reader{header, f, footer}

		for _, k := range pieces {
			_, err = io.Copy(w, k)
			if err != nil {
				w.CloseWithError(nil)
				return
			}
		}
		f.Close()
		w.Close()
	}()

	http_header := make(http.Header)
	http_header.Add("Content-Type", "multipart/form-data; boundary="+boundary)

	postRequest := &http.Request{
		Method:        "POST",
		URL:           real_url,
		Host:          apiHost,
		Header:        http_header,
		Body:          r,
		ContentLength: body_len,
	}
	return postRequest, nil
}

// Example:
// r.Upload("thumb.jpg", "image/jpeg")
func (request *Request) Upload(filename string, filetype string) (response *Response, err error) {
	postRequest, err := request.buildPost(uploadEndpoint, filename, filetype)
	if err != nil {
		return nil, err
	}
	return sendPost(postRequest)
}

func (request *Request) Replace(filename string, filetype string) (response *Response, err error) {
	postRequest, err := request.buildPost(replaceEndpoint, filename, filetype)
	if err != nil {
		return nil, err
	}
	return sendPost(postRequest)
}

func sendPost(postRequest *http.Request) (response *Response, err error) {
	// Create and use TCP connection (lifted mostly wholesale from http.send)
	client := http.DefaultClient
	resp, err := client.Do(postRequest)

	if err != nil {
		return nil, err
	}
	rawBody, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	var r Response
	err = xml.Unmarshal(rawBody, &r)

	return &r, err
}

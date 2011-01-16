package flickr

import (
	"io"
	"os"
	"fmt"
	"net"
	"http"
	"sort"
	"bufio"
	"bytes"
	"io/ioutil"
	"crypto/md5"
)

const (
	endpoint        = "http://api.flickr.com/services/rest/?"
	uploadEndpoint  = "http://api.flickr.com/services/upload/"
	replaceEndpoint = "http://api.flickr.com/services/replace/"
	apiHost         = "api.flickr.com"
)

type Request struct {
	ApiKey string
	Method string
	Args   map[string]string
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() os.Error { return nil }

type Error string

func (e Error) String() string {
	return string(e)
}

func (request *Request) Sign(secret string) {
	args := request.Args

	sorted_keys := make([]string, len(args)+2)

	args["api_key"] = request.ApiKey
	args["method"] = request.Method

	// Sort array keys
	i := 0
	for k, _ := range args {
		sorted_keys[i] = k
		i++
	}
	sort.SortStrings(sorted_keys)

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
	args["api_key"] = "", false
	args["method"] = "", false

	// Have the full string, now hash
	hash := md5.New()
	hash.Write([]byte(s))

	// Add api_sig as one of the args
	args["api_sig"] = fmt.Sprintf("%x", hash.Sum())
}

func (request *Request) Execute() (response string, ret os.Error) {
	args := request.Args

	if request.ApiKey == "" || request.Method == "" {
		return "", Error("Need both API key and method")
	}

	args["api_key"] = request.ApiKey
	args["method"] = request.Method

	s := endpoint + encodeQuery(args)

	res, _, err := http.Get(s)
	defer res.Body.Close()
	if err != nil {
		return "", err
	}

	body, _ := ioutil.ReadAll(res.Body)
	return string(body), nil
}

func encodeQuery(args map[string]string) string {
	i := 0
	s := bytes.NewBuffer(nil)
	for k, v := range args {
		if i != 0 {
			s.WriteString("&")
		}
		i++
		s.WriteString(k + http.URLEscape(v))
	}
	return s.String()
}

func (request *Request) buildPost(url string, filename string, filetype string) (*http.Request, os.Error) {
	f, err := os.Open(filename, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	request.Args["api_key"] = request.ApiKey

	boundary := "----###---###--flickr-go-rules"
	end := "\r\n"

	body := bytes.NewBuffer(nil)
	for k, v := range request.Args {
		body.WriteString("--" + boundary + end)
		body.WriteString("Content-Disposition: form-data; name=\"" + k + "\"" + end + end)
		body.WriteString(v + end)
	}

	body.WriteString("--" + boundary + end)
	body.WriteString("Content-Disposition: form-data; name=\"photo\"; filename=\"photo.jpg\"" + end)
	body.WriteString("Content-Type: " + filetype + end + end)

	// Write file
	_, err = io.Copy(body, f)
	if err != nil {
		return nil, err
	}

	body.WriteString(end)
	body.WriteString("--" + boundary + "--" + end)

	postRequest := new(http.Request)
	postRequest.Method = "POST"
	postRequest.RawURL = url
	postRequest.Host = apiHost
	postRequest.Header = map[string]string{
		"Content-Type": "multipart/form-data; boundary=" + boundary + end,
	}

	postRequest.Body = nopCloser{body}
	postRequest.ContentLength = int64(body.Len())
	return postRequest, nil
}

func (request *Request) Upload(filename string, filetype string) (response string, err os.Error) {
	postRequest, err := request.buildPost(uploadEndpoint, filename, filetype)
	if err != nil {
		return "", err
	}

	return sendPost(postRequest)
}

func (request *Request) Replace(filename string, filetype string) (response string, err os.Error) {
	postRequest, err := request.buildPost(replaceEndpoint, filename, filetype)
	if err != nil {
		return "", err
	}
	return sendPost(postRequest)
}

func sendPost(postRequest *http.Request) (body string, err os.Error) {
	// Create and use TCP connection (lifted mostly wholesale from http.send)
	conn, err := net.Dial("tcp", "", "api.flickr.com:80")
	defer conn.Close()

	if err != nil {
		return "", err
	}
	postRequest.Write(conn)

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, postRequest.Method)
	if err != nil {
		return "", err
	}
	rawBody, _ := ioutil.ReadAll(resp.Body)

	return string(rawBody), nil
}

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
	upload_endpoint = "http://api.flickr.com/services/upload/"
	api_host        = "api.flickr.com"
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

// So we can return custom errors
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
	s := ""
	i := 0
	for k, v := range args {
		if i != 0 {
			s += "&"
		}
		i++
		s += fmt.Sprintf("%s=%s", k, http.URLEscape(v))
	}
	return s
}

func (request *Request) Upload(filename string, filetype string) (response string, err os.Error) {
	photo_file, error := ioutil.ReadFile(filename)
	if error != nil {
		return "", error
	}

	request.Args["api_key"] = request.ApiKey

	boundary := "----###---###--flickr-go-rules"
	end := "\r\n"

	post_body := ""
	for k, v := range request.Args {
		post_body += "--" + boundary + end
		post_body += "Content-Disposition: form-data; name=\"" + k + "\"" + end + end
		post_body += v + end
	}

	post_body += "--" + boundary + end
	post_body += "Content-Disposition: form-data; name=\"photo\"; filename=\"photo.jpg\"" + end
	post_body += "Content-Type: " + filetype + end + end
	post_body += string(photo_file) + end
	post_body += "--" + boundary + "--" + end

	post_req := new(http.Request)
	post_req.Method = "POST"
	post_req.RawURL = upload_endpoint
	post_req.Host = api_host
	post_req.Header = map[string]string{
		"Content-Type": "multipart/form-data; boundary=" + boundary + end,
	}

	post_req.Body = nopCloser{bytes.NewBufferString(post_body)}
	post_req.ContentLength = int64(len(post_body))

	// Create and use TCP connection (lifted mostly wholesale from http.send)
	conn, err := net.Dial("tcp", "", "api.flickr.com:80")
	defer conn.Close()

	if err != nil {
		return "", err
	}
	post_req.Write(conn)

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, post_req.Method)
	if err != nil {
		return "", err
	}
	body, _ := ioutil.ReadAll(resp.Body)

	return string(body), nil
}

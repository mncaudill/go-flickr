package flickr

import (
	"os"
	"fmt"
	"http"
	"sort"
	"io/ioutil"
	"crypto/md5"
)

const (
	endpoint = "http://api.flickr.com/services/rest/?"
)

type Request struct {
	ApiKey string
	Method string
	Args   map[string]string
}

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
		s += fmt.Sprintf("%s%s", key, args[key])
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

	s := endpoint
	i := 0
	for k, v := range args {
		if i != 0 {
			s += "&"
		}
		i++
		s += fmt.Sprintf("%s=%s", k, http.URLEscape(v))
	}

	res, _, err := http.Get(s)
	defer res.Body.Close()
	if err != nil {
		return "", err
	}

	body, _ := ioutil.ReadAll(res.Body)
	return string(body), nil
}

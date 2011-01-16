package main

import (
	"./flickr"
	"fmt"
)

func main() {
	r := new(flickr.Request)
	r.ApiKey = "YOURAPIKEYHERE"

	r.Args = map[string]string{
		"auth_token": "72157625826023910-2ab8f4da623e6ca8",
		"title":      "Good mornin'",
	}

	r.Sign("YOURAPISECRETHERE")
	resp, err := r.Upload("thumb.jpg", "image/jpeg")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(resp)
}

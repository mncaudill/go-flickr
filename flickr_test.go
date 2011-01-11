package main

import "./flickr"
import "fmt"

func main() {
	r := new(flickr.Request)
	r.ApiKey = "APIKEYGOESHERE"
	r.Method = "flickr.photos.getInfo"

	r.Args = make(map[string]string, 10)
	r.Args["photo_id"] = "5336400553"

	r.Sign("APISECRETGOESHERE")
	body, err := r.Execute()
	if err != nil {
		fmt.Println("Something went wrong:", err)
	}
	fmt.Println(body)
}

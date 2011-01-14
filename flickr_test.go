package main

import "./flickr"
import "fmt"


func main() {
	r := new(flickr.Request)
	r.ApiKey = "APIKEYHERE"
	r.Method = "flickr.photos.getInfo"

	r.Args = make(map[string]string, 10)
	r.Args["photo_id"] = "5336400553"

	r.Sign("APISECRETHERE")
	_, err := r.Execute()
	if err != nil {
		fmt.Println("Something went wrong:", err)
	}
//	fmt.Println(body)

	r.Upload("thumb.jpg")
}

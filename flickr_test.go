package main

import "./flickr"
import "fmt"


func main() {
	r := new(flickr.Request)
	r.ApiKey = "e17940558f4af5df0933c5b7aac53de0"
	r.Method = "flickr.photos.getInfo"

	r.Args = make(map[string]string, 10)
	r.Args["photo_id"] = "5336400553"

	r.Sign("e2a257c7891af67e")
	_, err := r.Execute()
	if err != nil {
		fmt.Println("Something went wrong:", err)
	}
//	fmt.Println(body)

	r.Upload("thumb.jpg")
}

all:	flickr_test

flickr_test: flickr.8 flickr_test.8 
	8l -o flickr_test flickr_test.8

flickr.8:	flickr.go
	8g flickr.go

flickr_test.8:	flickr_test.go
	8g flickr_test.go

clean:
	rm -f flickr_test *.8


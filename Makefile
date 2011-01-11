CC= 8g
LINKER= 8l

all:	flickr_test

flickr_test: flickr.8 flickr_test.8 
	$(LINKER) -o flickr_test flickr_test.8

flickr.8:	flickr.go
	$(CC) flickr.go

flickr_test.8:	flickr_test.go
	$(CC) flickr_test.go

clean:
	rm -f flickr_test *.5 *.6 *.8


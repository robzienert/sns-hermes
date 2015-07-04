# -*- Makefile -*-

version=0.0.1
deb_file=sqs-webhook_${version}_amd64.deb

build: test package

test:
	go test

package:
	goxc -bc="linux" -d build -pv="${version}"

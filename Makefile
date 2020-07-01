.PHONY : build deploy test

all: build deploy

build:
	GOOS=linux go build -ldflags '-s' main.go
	zip function.zip main 

deploy:
	aws lambda update-function-code \
		--function-name create_thumbnail \
	 	--zip-file fileb://function.zip

test:
	aws s3 rm s3://$(AWS_BUCKET)/source/designs/1/image.jpg
	aws s3 cp ./test/image.jpg s3://$(AWS_BUCKET)/source/designs/1/image.jpg

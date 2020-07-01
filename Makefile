all: build deploy

build:
	GOOS=linux go build -ldflags '-s' main.go
	zip function.zip main 

deploy:
	aws lambda update-function-code --function-name create_thumbnail  --zip-file fileb://function.zip
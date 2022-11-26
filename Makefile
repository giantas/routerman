.PHONY : build tidy

build:
	@go build -o routerman .

tidy:
	@go mod tidy
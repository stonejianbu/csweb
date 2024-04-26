.PHONY: gen-proto

gen-proto:
	protoc -I./third_party -I./protos --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative --go_out=./protos --go-grpc_out=./protos protos/*/*.proto
.PHONY: proto_regen ## Regenerate protobuf artifacts
proto_regen:
	find proto -name "*.proto" -exec protoc --go_out=. --go_opt=paths=source_relative {} \;
	find proto -name "*.pb.go" | sed 's/proto\///' | xargs -I {} mv proto/{} {}
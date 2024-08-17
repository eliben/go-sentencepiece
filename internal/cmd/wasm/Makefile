.PHONY: build serve clean

build:
	mkdir -p embed_data
	wget https://github.com/google/gemma_pytorch/raw/main/tokenizer/tokenizer.model -O embed_data/tokenizer.model
	GOOS=js GOARCH=wasm go build -o assets/gospm.wasm main.go

serve:
	go run github.com/eliben/static-server@latest -port 8873 assets

clean:
	rm -rf embed_data assets/gospm.wasm

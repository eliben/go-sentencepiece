# go-sentencepiece

## Developing

The `sentencepiece_model.proto` file is vendored from
https://github.com/google/sentencepiece

To re-generate the `*.pb.go` file from it, run:

```
$ protoc --go_out=. sentencepiece_model.proto
```

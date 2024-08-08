# go-sentencepiece

This is a pure Go implementation of encoding text with
the [SentencePiece tokenizer](https://github.com/google/sentencepiece).

"Encoding" is the operation used to split text into tokens, using
a trained tokenizer model.

SentencePiece is a general family of tokenizers that is configured
by a protobuf configuration file. This repository currently focuses
on implementing just the functionality required to reproduce the
tokenization of [Gemma models](https://ai.google.dev/gemma) (the same
tokenizer is used for Google's proprietary Gemini family of models).
Specifically, it only implements BPE tokenization since this is what
Gemma uses.

## Tokenizer configuration

The configuration file for the tokenizer describes a trained tokenizer
model. It is not part of this repository. Please fetch it from the
[official Gemma implementation repository](https://github.com/google/gemma_pytorch/tree/main/tokenizer).
The `NewEncoder` constructor will expect a local path to this file.

## Developing

A protobuf is used to configure the tokenizer. The structure of the
protobuf is described by the `sentencepiece_model.proto` file, which
is vendored from https://github.com/google/sentencepiece

To re-generate the `*.pb.go` file from it, run:

```
$ protoc --go_out=. sentencepiece_model.proto
```

The configuration protobuf itself is obtained as described in the
[Tokenizer configuration](#tokenizer-configuration) section. All
tests require the `MODELPATH` env var to point to a local
copy of the tokenizer configuration file.

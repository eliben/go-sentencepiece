# go-sentencepiece

[![Go Reference](https://pkg.go.dev/badge/github.com/eliben/go-sentencepiece.svg)](https://pkg.go.dev/github.com/eliben/go-sentencepiece)

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

## Current status

The initial version of `go-sentencepiece` aims to achieve correctness,
with the original [SentencePiece](https://github.com/google/sentencepiece)
(accessed through its [Python bindings](https://pypi.org/project/sentencepiece/))
for reference. `go-sentencepiece` is tested to produce an identical sequence
of tokens for a range for textual files.

No effort has been spent on optimization yet; `go-sentencepiece` uses a
naive quadratic algorithm for BPE tokenization, and will run slowly for
large inputs. Optimization is being worked on now - expect much better
performance in the next version.

## Tokenizer configuration

The configuration file for the tokenizer is a protobuf (structured
data, serialized in the [protocol buffer format](https://protobuf.dev/))
that describes a trained tokenizer model; it includes
the complete learned vocabulary used for tokenization, as well as
other configuration information.

It is not part of this repository. Please fetch it from the
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

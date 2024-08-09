package sentencepiece_test

import (
	"fmt"
	"log"
	"os"

	"github.com/eliben/go-sentencepiece"
)

func ExampleEncode() {
	protoFile := os.Getenv("MODELPATH")
	if protoFile == "" {
		log.Println("Need MODELPATH env var to run example")
		return
	}

	enc, err := sentencepiece.NewEncoderFromPath(protoFile)
	if err != nil {
		log.Fatal(err)
	}

	text := "Encoding produces tokens that LLMs can learn and understand"
	tokens := enc.Encode(text)

	for _, token := range tokens {
		fmt.Println(token)
	}
}

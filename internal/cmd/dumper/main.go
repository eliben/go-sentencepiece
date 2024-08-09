package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"unicode"

	"github.com/eliben/go-sentencepiece"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

func main() {
	fDumpAll := flag.Bool("dumpall", false, "dump entire model proto")
	fFindUni := flag.Bool("finduni", false, "find unicode runes not in pieces")
	fEncodeFile := flag.String("encodefile", "", "file name to open and encode")
	flag.Parse()

	modelPath := os.Getenv("MODELPATH")
	if modelPath == "" {
		log.Fatal("Need MODELPATH env var to run")
	}

	b, err := ioutil.ReadFile(modelPath)
	if err != nil {
		log.Fatal(err)
	}

	var model sentencepiece.ModelProto
	err = proto.Unmarshal(b, &model)
	if err != nil {
		log.Fatal(err)
	}

	if *fDumpAll {
		fmt.Println(prototext.Format(&model))
	} else if *fFindUni {
		pieces := make(map[string]int)
		for i, piece := range model.GetPieces() {
			pieces[piece.GetPiece()] = i
		}

		for r := rune(0); r <= unicode.MaxRune; r++ {
			if unicode.IsPrint(r) {
				if _, found := pieces[string(r)]; !found {
					fmt.Printf("not in pieces: %U %q\n", r, string(r))
				}
			}
		}
	} else if *fEncodeFile != "" {
		enc, err := sentencepiece.NewEncoderFromPath(modelPath)
		if err != nil {
			log.Fatal(err)
		}

		b, err := ioutil.ReadFile(*fEncodeFile)
		if err != nil {
			log.Fatal(err)
		}

		tokens := enc.Encode(string(b))
		for _, t := range tokens {
			fmt.Println(t.ID)
		}
	}
}

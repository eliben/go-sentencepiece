package main

// Command dumper is a debugging utility for internal use. It helps explore
// the model proto and compare results with other tools.

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"unicode"

	"github.com/eliben/go-sentencepiece"
	"github.com/eliben/go-sentencepiece/internal/model"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

func main() {
	fDumpAll := flag.Bool("dumpall", false, "dump entire model proto")
	fFindUni := flag.Bool("finduni", false, "find unicode runes not in pieces")
	fFindBytes := flag.Bool("findbytes", false, "show all byte pieces with their IDs")
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

	var protomodel model.ModelProto
	err = proto.Unmarshal(b, &protomodel)
	if err != nil {
		log.Fatal(err)
	}

	if *fDumpAll {
		fmt.Println(prototext.Format(&protomodel))
	} else if *fFindBytes {
		for i, piece := range protomodel.GetPieces() {
			if piece.GetType() == model.ModelProto_SentencePiece_BYTE {
				fmt.Printf("%5d: %s\n", i, piece.GetPiece())
			}
		}

	} else if *fFindUni {
		pieces := make(map[string]int)
		for i, piece := range protomodel.GetPieces() {
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

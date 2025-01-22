package pgn

import (
	"strings"
	"os"
	"io/ioutil"
	"log"

	"gopkg.in/freeeve/pgn.v1"
)

func PgnParseFromString(pgnString string) []string {
	r := strings.NewReader(pgnString)

	ps := pgn.NewPGNScanner(r)

	var fenList []string
	for ps.Next() {
		game, err := ps.Scan()
		if err != nil {
			log.Fatal(err)
		}

		b := pgn.NewBoard()
		for _, move := range game.Moves {
			b.MakeMove(move)
			fen := b.String()
			fenList = append(fenList, fen)
		}
	}

	return fenList
}

func ReadContentFromFile(filePath string) (string, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func PgnParseFromFile(filepath string) []string {
	var fenList []string

	f, err := os.Open(filepath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()


	ps := pgn.NewPGNScanner(f)

	for ps.Next() {
		game, err := ps.Scan()
		if err != nil {
			log.Fatal(err)
		}

		b := pgn.NewBoard()
		for _, move := range game.Moves {
			b.MakeMove(move)
			fen := b.String()
			fenList = append(fenList, fen)
		}
	}

	return fenList
}

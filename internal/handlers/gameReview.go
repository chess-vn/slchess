package handlers

import (
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/bucket-sort/slchess/pkg/utils"
	"github.com/notnil/chess"
	"github.com/notnil/chess/uci"
	// "fmt"
	// "log"
	// "runtime"
	// "sync"
	// "time"
	// "github.com/freeeve/uci"
)

func GameReviewHandler() {
	startTime := time.Now()

	// PGN cần phân tích
	pgnString := `[Event "ch-UZB 1st League 2014"]
[Site "Tashkent UZB"]
[Date "2014.03.01"]
[Round "1.5"]
[White "Abdusattorov,Nodirbek"]
[Black "Alikulov,Elbek"]
[Result "1-0"]
[WhiteElo "2024"]
[BlackElo "2212"]
[ECO "B28"]

1.e4 c5 2.Nf3 a6 3.d3 g6 4.g3 Bg7 5.Bg2 b5 6.O-O Bb7 7.c3 e5 8.a3 Ne7 9.b4 d6
10.Nbd2 O-O 11.Nb3 Nd7 12.Be3 Rc8 13.Rc1 h6 14.Nfd2 f5 15.f4 Kh7 16.Qe2 cxb4
17.axb4 exf4 18.Bxf4 Rxc3 19.Rxc3 Bxc3 20.Bxd6 Qb6+ 21.Bc5 Nxc5 22.bxc5 Qe6
23.d4 Rd8 24.Qd3 Bxd2 25.Nxd2 fxe4 26.Nxe4 Nf5 27.d5 Qe5 28.g4 Ne7 29.Rf7+ Kg8
30.Qf1 Nxd5 31.Rxb7 Qd4+ 32.Kh1 Rf8 33.Qg1 Ne3 34.Re7 a5 35.c6 a4 36.Qxe3 Qxe3
37.Nf6+ Rxf6 38.Rxe3 Rd6 39.h4 Rd1+ 40.Kh2 b4 41.c7  1-0
`

	// Parse FEN từ PGN
	fenList := utils.PgnParseFromString(pgnString)
	if len(fenList) == 0 {
		log.Println("No FEN positions found in PGN.")
		return
	}

	// Tao engine
	eng, err := uci.New("D:/WorkSpace/GoLang/stockfish/stockfish-windows-x86-64-avx2.exe")
	if err != nil {
		panic(err)
	}
	defer eng.Close()

	if err := eng.Run(uci.CmdUCI, uci.CmdIsReady, uci.CmdUCINewGame); err != nil {
		panic(err)
	}

	game := chess.NewGame()
	fmt.Println(reflect.TypeOf(game.Position()))

	cmdPos := uci.CmdPosition{Position: game.Position()}
	cmdGo := uci.CmdGo{Depth: 10}
	if err := eng.Run(cmdPos, cmdGo); err != nil {
		panic(err)
	}
	// move := eng.SearchResults().BestMove
	// if err := game.Move(move); err != nil {
	// 	panic(err)
	// }

	// fmt.Println(game.String())
	elapsed := time.Since(startTime)
	fmt.Printf("Time taken: %s\n", elapsed)

	// 	startTime := time.Now()

	// 	// PGN cần phân tích
	// 	pgnString := `[Event "ch-UZB 1st League 2014"]
	// [Site "Tashkent UZB"]
	// [Date "2014.03.01"]
	// [Round "1.5"]
	// [White "Abdusattorov,Nodirbek"]
	// [Black "Alikulov,Elbek"]
	// [Result "1-0"]
	// [WhiteElo "2024"]
	// [BlackElo "2212"]
	// [ECO "B28"]

	// 1.e4 c5 2.Nf3 a6 3.d3 g6 4.g3 Bg7 5.Bg2 b5 6.O-O Bb7 7.c3 e5 8.a3 Ne7 9.b4 d6
	// 10.Nbd2 O-O 11.Nb3 Nd7 12.Be3 Rc8 13.Rc1 h6 14.Nfd2 f5 15.f4 Kh7 16.Qe2 cxb4
	// 17.axb4 exf4 18.Bxf4 Rxc3 19.Rxc3 Bxc3 20.Bxd6 Qb6+ 21.Bc5 Nxc5 22.bxc5 Qe6
	// 23.d4 Rd8 24.Qd3 Bxd2 25.Nxd2 fxe4 26.Nxe4 Nf5 27.d5 Qe5 28.g4 Ne7 29.Rf7+ Kg8
	// 30.Qf1 Nxd5 31.Rxb7 Qd4+ 32.Kh1 Rf8 33.Qg1 Ne3 34.Re7 a5 35.c6 a4 36.Qxe3 Qxe3
	// 37.Nf6+ Rxf6 38.Rxe3 Rd6 39.h4 Rd1+ 40.Kh2 b4 41.c7  1-0
	// `

	// 	// Parse FEN từ PGN
	// 	fenList := utils.PgnParseFromString(pgnString)
	// 	if len(fenList) == 0 {
	// 		log.Println("No FEN positions found in PGN.")
	// 		return
	// 	}

	// 	// Số lượng engine và độ sâu phân tích
	// 	numEngines := runtime.NumCPU()
	// 	depth := 10

	// 	// Chia FEN thành các nhóm
	// 	fenGroups := make([][]string, numEngines)
	// 	for i, fen := range fenList {
	// 		groupIndex := i % numEngines
	// 		fenGroups[groupIndex] = append(fenGroups[groupIndex], fen)
	// 	}

	// 	resultsChan := make(chan uci.Results, len(fenList))
	// 	var wg sync.WaitGroup

	// 	// Hàm để phân tích một nhóm FEN
	// 	analyzeGroup := func(group []string) {
	// 		defer wg.Done()

	// 		eng, err := uci.NewEngine("D:/WorkSpace/GoLang/stockfish/stockfish-windows-x86-64-avx2.exe")
	// 		if err != nil {
	// 			log.Printf("Error creating engine: %v", err)
	// 			return
	// 		}
	// 		defer eng.Close()

	// 		// Thiết lập options cho engine
	// 		err = eng.SetOptions(uci.Options{
	// 			Hash:    128,
	// 			MultiPV: 1,
	// 		})
	// 		if err != nil {
	// 			log.Printf("Error setting options: %v", err)
	// 			return
	// 		}

	// 		// Phân tích từng FEN trong nhóm
	// 		for _, fen := range group {
	// 			// Thiết lập FEN
	// 			err = eng.SetFEN(fen)
	// 			if err != nil {
	// 				log.Printf("Error setting FEN %s: %v", fen, err)
	// 				continue
	// 			}

	// 			// Phân tích
	// 			resultOpts := uci.HighestDepthOnly | uci.IncludeUpperbounds | uci.IncludeLowerbounds
	// 			results, err := eng.GoDepth(depth, resultOpts)
	// 			if err != nil {
	// 				log.Printf("Error analyzing position %s: %v", fen, err)
	// 				continue
	// 			}

	// 			resultsChan <- *results
	// 		}
	// 	}

	// 	// Khởi chạy các goroutine để phân tích từng nhóm FEN
	// 	for _, group := range fenGroups {
	// 		wg.Add(1)
	// 		go analyzeGroup(group)
	// 	}

	// 	// Goroutine để đóng channel sau khi tất cả workers hoàn thành
	// 	go func() {
	// 		wg.Wait()
	// 		close(resultsChan)
	// 	}()

	// 	// Thu thập kết quả từ channel
	// 	var allResults []uci.Results
	// 	for result := range resultsChan {
	// 		allResults = append(allResults, result)
	// 	}

	// 	// In kết quả
	// 	for i, result := range allResults {
	// 		fmt.Printf("Result %d: %v\n", i+1, result)
	// 	}

	// 	// Thời gian chạy
	// 	elapsed := time.Since(startTime)
	// 	fmt.Printf("Time taken: %s\n", elapsed)
}

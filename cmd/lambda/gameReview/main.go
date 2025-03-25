package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chess-vn/slchess/pkg/utils"
	"github.com/freeeve/uci"
)

func GameReviewHandler() {
	startTime := time.Now()
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
37.Nf6+ Rxf6 38.Rxe3 Rd6 39.h4 Rd1+ 40.Kh2 b4 41.c7  1-0`

	fenList := utils.PgnParseFromString(pgnString)

	// Tạo engine một lần và sử dụng cho tất cả goroutines
	eng, err := uci.NewEngine("d:/Binh/stockfish/stockfish-windows-x86-64-avx2.exe")
	if err != nil {
		log.Fatal(err)
	}
	defer eng.Close()

	// Thiết lập options cho engine
	eng.SetOptions(uci.Options{
		Threads: 2,
		Hash:    128,
		Ponder:  true,
		OwnBook: true,
		MultiPV: 5,
	})
	depth := 25

	// Channel để nhận kết quả từ các goroutines
	resultsChan := make(chan uci.Results, len(fenList))

	// WaitGroup để đảm bảo tất cả goroutines hoàn thành
	var wg sync.WaitGroup

	// Mutex để đồng bộ hóa việc truy cập vào engine
	var engineMutex sync.Mutex

	// Khởi chạy goroutine cho mỗi FEN position
	for _, fen := range fenList {
		wg.Add(1)
		go func(fen string) {
			defer wg.Done()

			// Lock mutex khi sử dụng engine
			engineMutex.Lock()
			eng.SetFEN(fen)
			resultOpts := uci.HighestDepthOnly | uci.IncludeUpperbounds | uci.IncludeLowerbounds
			results, err := eng.GoDepth(depth, resultOpts)
			engineMutex.Unlock()

			if err != nil {
				log.Printf("Error analyzing position %s: %v", fen, err)
				return
			}

			resultsChan <- *results
		}(fen)
	}

	// Goroutine để đóng channel sau khi tất cả workers hoàn thành
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Thu thập kết quả từ channel
	var allResults []uci.Results
	for result := range resultsChan {
		allResults = append(allResults, result)
	}

	// In kết quả
	for _, result := range allResults {
		fmt.Println(result)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("Time taken: %s\n", elapsed)
}

func main() {
	GameReviewHandler()
}

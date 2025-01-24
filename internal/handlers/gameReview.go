package handlers

import (
	"fmt"

	"github.com/freeeve/uci"
	"github.com/bucket-sort/slchess/pkg/utils"

	"time"
	"sync"
	"log"
)

func GameReviewHandler() {
	startTime := time.Now()
    pgnString := `[Event "Casual Game"]
[Site "Online"]
[Date "2025.01.22"]
[Round "?"]
[White "Player 1"]
[Black "Player 2"]
[Result "1-0"]

1. e4 e5 2. Qh5 Nc6 3. Bc4 Bc5 4. Qxf7# 1-0
`

    fenList := utils.PgnParseFromString(pgnString)
    
    // Tạo engine một lần và sử dụng cho tất cả goroutines
    eng, err := uci.NewEngine("d:/Binh/stockfish/stockfish-windows-x86-64-avx2.exe")
    if err != nil {
        log.Fatal(err)
    }
    defer eng.Close()

    // Thiết lập options cho engine
    eng.SetOptions(uci.Options{
        Hash:     128,
        Ponder:   true,
        OwnBook:  true,
        MultiPV:  5,
    })
    depth := 20

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
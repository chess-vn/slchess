package stofinet

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type Pv struct {
	Cp    int
	Moves string
}

type Evaluation struct {
	Fen    string
	Depth  int
	Knodes int
	Pvs    []Pv
}

func parsePvsLines(lines []string) Evaluation {
	// Improved regex pattern
	re := regexp.MustCompile(`depth (\d+).*?score cp (-?\d+).*?nodes (\d+).*?pv (.+)`)

	var eval Evaluation
	eval.Pvs = []Pv{}

	for _, line := range lines {
		fmt.Println("Processing Line:", line) // Debugging step

		match := re.FindStringSubmatch(line)
		if match == nil {
			fmt.Println("No match found for line:", line)
			continue
		}

		depth, err1 := strconv.Atoi(match[1])
		cp, err2 := strconv.Atoi(match[2])
		nodes, err3 := strconv.Atoi(match[3])
		moves := strings.TrimSpace(match[4])

		if err1 != nil || err2 != nil || err3 != nil {
			fmt.Println("Error converting values:", err1, err2, err3)
			continue
		}

		// Set depth and knodes once
		if eval.Depth == 0 {
			eval.Depth = depth
			eval.Knodes = nodes / 1000
		}

		// Append move sequence
		eval.Pvs = append(eval.Pvs, Pv{
			Cp:    cp,
			Moves: moves,
		})
	}

	return eval
}

// runStockfish runs the Stockfish engine with the given FEN and depth

func runStockfish(path string, fen string, depth int) ([]string, error) {
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	writer := bufio.NewWriter(stdin)
	reader := bufio.NewScanner(stdout)

	options := []string{
		"uci",
		"setoption name Threads value 2",
		"setoption name Hash value 256",
		"setoption name MultiPV value 3",
		"isready",
	}
	for _, option := range options {
		fmt.Fprintln(writer, option)
	}
	writer.Flush()

	// Wait for Stockfish to be ready
	for reader.Scan() {
		if reader.Text() == "readyok" {
			break
		}
	}

	// Set position and start analysis
	fmt.Fprintln(writer, "position fen "+fen)
	fmt.Fprintf(writer, "go depth %d\n", depth)
	writer.Flush()

	// Read Stockfish output and extract MultiPV lines
	stopStr := fmt.Sprintf("info depth %d", depth)
	var pvLines []string
	for reader.Scan() {
		line := reader.Text()
		if strings.Contains(line, "bestmove") {
			break // Stop reading once bestmove is received
		}
		if strings.Contains(line, stopStr) && strings.Contains(line, " multipv ") {
			pvLines = append(pvLines, line)
		}
	}

	stdin.Close()
	stdout.Close()
	cmd.Wait()
	return pvLines, nil
}

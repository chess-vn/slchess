package stofinet

import (
	"fmt"

	"github.com/freeeve/uci"
)

func (client *Client) Evaluate(fen string, depth int) (*uci.Results, error) {
	client.engine.SetFEN(fen)
	resultOpts := uci.HighestDepthOnly | uci.IncludeLowerbounds | uci.IncludeUpperbounds
	results, err := client.engine.GoDepth(depth, resultOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to go depth %d: %w", depth, err)
	}
	return results, nil
}

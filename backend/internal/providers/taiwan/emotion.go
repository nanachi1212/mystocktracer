package taiwan

import (
	"context"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/marketemotion"
)

func (c *Client) MarketEmotion(ctx context.Context, now time.Time) (marketemotion.TaiwanMarketEmotion, error) {
	breadth, err := c.MarketBreadth(ctx, now)
	if err != nil {
		return marketemotion.TaiwanMarketEmotion{}, err
	}
	return marketemotion.CalculateTaiwanMarketEmotion(breadth), nil
}

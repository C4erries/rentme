package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

type ModelMetrics struct {
	MAE       float64 `json:"mae"`
	RMSE      float64 `json:"rmse"`
	TrainSize int     `json:"train_size"`
	TestSize  int     `json:"test_size"`
}

type MLMetrics struct {
	ShortTerm ModelMetrics `json:"short_term"`
	LongTerm  ModelMetrics `json:"long_term"`
}

// MetricsClient fetches ML model metrics from the pricing service.
type MetricsClient struct {
	Endpoint string
	Client   *http.Client
	Logger   *slog.Logger
}

func (c *MetricsClient) Fetch(ctx context.Context) (*MLMetrics, error) {
	if c == nil || c.Client == nil {
		return nil, errors.New("ml metrics: http client not configured")
	}
	if c.Endpoint == "" {
		return nil, errors.New("ml metrics: endpoint not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
			err = fmt.Errorf("ml metrics: pricing service timeout (%s)", c.Endpoint)
		} else {
			err = fmt.Errorf("ml metrics: pricing service unavailable (%s)", c.Endpoint)
		}
		c.logError("metrics request failed", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		err := fmt.Errorf("ml metrics: pricing service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		c.logError("metrics returned error", err)
		return nil, err
	}

	var metrics MLMetrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		c.logError("metrics decode failed", err)
		return nil, err
	}
	return &metrics, nil
}

func (c *MetricsClient) logError(msg string, err error) {
	if c.Logger != nil {
		c.Logger.Error(msg, "error", err)
	}
}

package llm

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const maxResponseBytes = 4 << 20

func providerClient() *http.Client {
	return &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

// Provider errors may echo credentials or prompts. Never persist their raw body.
func responseBody(resp *http.Response) ([]byte, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("模型服务返回状态 %d，请检查服务地址、凭据和模型", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, errors.New("模型响应读取失败")
	}
	if len(data) > maxResponseBytes {
		return nil, errors.New("模型响应超出大小限制")
	}
	return data, nil
}

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type tokenCache struct {
	token    string
	expireAt time.Time
	mu       sync.RWMutex
}

func (p *DingTalkRobotProvider) getAccessToken(ctx context.Context) (string, error) {

	// ---------- Fast Path ----------
	p.tokenCache.mu.RLock()
	if p.tokenCache.token != "" &&
		time.Now().Before(p.tokenCache.expireAt) {

		token := p.tokenCache.token
		p.tokenCache.mu.RUnlock()
		return token, nil
	}
	p.tokenCache.mu.RUnlock()

	// ---------- Refresh ----------
	p.tokenCache.mu.Lock()
	defer p.tokenCache.mu.Unlock()

	// Double Check
	if p.tokenCache.token != "" &&
		time.Now().Before(p.tokenCache.expireAt) {
		return p.tokenCache.token, nil
	}

	url := fmt.Sprintf(
		"https://oapi.dingtalk.com/gettoken?appkey=%s&appsecret=%s",
		p.appKey,
		p.appSecret,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf(
			"dingtalk gettoken failed [%d]: %s",
			result.ErrCode,
			result.ErrMsg,
		)
	}

	p.tokenCache.token = result.AccessToken

	expire := result.ExpiresIn

	if expire <= 300 {
		expire = 7200
	}

	// 提前5分钟刷新
	p.tokenCache.expireAt = time.Now().Add(
		time.Duration(expire-300) * time.Second,
	)

	return result.AccessToken, nil
}

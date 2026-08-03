package middleware

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// TokenAuthMiddleware 全局 Token 校验中间件
func TokenAuthMiddleware(validToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// 🔒 1. 检查服务端配置：如果未配置 Token，出于安全防护默认拒绝所有请求
		if validToken == "" {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(Response{
				Code:    http.StatusForbidden,
				Message: "Forbidden: API token is not configured on the server",
			})
			return
		}

		// 2. 从 Header 获取 X-Api-Token 或 X-Jenkins-Token
		clientToken := r.Header.Get("X-Api-Token")
		// 🔒 3. 校验 Token：客户端未提供或 Token 不匹配，返回 401 Unauthorized
		if clientToken != validToken {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(Response{
				Code:    http.StatusUnauthorized,
				Message: "Unauthorized: Invalid or missing API Token",
			})
			return
		}

		// 校验通过，放行继续处理请求
		next.ServeHTTP(w, r)
	})
}

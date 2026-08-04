package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
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

		// 2. 尝试从多种方式获取客户端 Token
		clientToken := extractToken(r)

		// 🔒 3. 校验 Token：客户端未提供或 Token 不匹配，返回 401 Unauthorized
		if clientToken == "" || clientToken != validToken {
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

// extractToken 从请求中提取 Token，支持：
// 1. Authorization: Bearer <token>
// 2. X-Api-Token: <token>
// 3. X-Jenkins-Token: <token> (如果有需要)
func extractToken(r *http.Request) string {
	// 优先检查标准 Authorization: Bearer <token>
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// 忽略大小写判断是否以 "Bearer " 开头
		if len(authHeader) > 7 && strings.EqualFold(authHeader[:7], "Bearer ") {
			return strings.TrimSpace(authHeader[7:])
		}
	}

	// 备选 1: 检查 X-Api-Token
	if apiToken := r.Header.Get("X-Api-Token"); apiToken != "" {
		return strings.TrimSpace(apiToken)
	}

	// 备选 2: 检查 X-Jenkins-Token (根据注释需要)
	if jenkinsToken := r.Header.Get("X-Jenkins-Token"); jenkinsToken != "" {
		return strings.TrimSpace(jenkinsToken)
	}

	return ""
}

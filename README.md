# ReadMe
 * Send Alert Message To Ding Robot & Dingding Webhook 
## config.yaml
* place config.yaml in config directory

```
# 全局安全认证配置
auth:
  api_token: "your-global-secret-token-2026"  # 所有接口通用的认证 Token
# 这里填入你的阿里云百炼 DeepSeek API Key
aliyun_deepseek_api_key: xx
server:
  port: "8080"
  #--------------------------------------
  # dingding 默认用户userid
  #   88888 对应钉钉用户名 
  #-----------------------
default_receiver_userid:
  - "88888"
default_receiver_email:
  - xx@gmail.com
# 默认推送的目标群 openConversationId 列表
# 产品小队群对应得openConversationId==
default_receiver_groupid:
   - xxx

providers:
  # 渠道 1: 钉钉应用私信
  dingtalk_robot:
    app_key: "*****"
    app_secret: "*****"
    enable_ding: true   # 启用DING功能
    ding_type: 1   # DING 类型：1-应用内DING(默认)，2-短信DING，3-电话DING
  # 🚀 Jenkins 发版专用的单独机器人
  jenkins_dingtalk_robot:
    app_key: "*****"
    app_secret: "*****"
    enable_ding: true   # 启用DING功能
    ding_type: 1   # DING 类型：1-应用内DING(默认)，2-短信DING，3-电话DING
  # 企业内部应用 - 发送群消息驱动
  dingtalk_app_group:
    app_key: "*****"
    app_secret: "*****"
    enable_ding: true   # 启用DING功能
    ding_type: 1   # DING 类型：1-应用内DING(默认)，2-短信DING，3-电话DING
  # 渠道 2: 钉钉群 Webhook
  dingtalk_webhook:
    webhook_url: "https://oapi.dingtalk.com/robot/send?access_token=*****"
    secret: "*****" # 选填：加签密钥

#  # 渠道 3: 邮件服务 (SMTP)
#  email:
#    smtp_host: "smtp.qq.com"
#    smtp_port: 465
#    smtp_user: "alert@yourcompany.com"
#    smtp_pass: "your_smtp_password_or_token"
#    from: "运维告警中心 <alert@yourcompany.com>"


```

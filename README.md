# ReadMe

## config.yaml
* place config.yaml in config directory

```
server:
  port: "8080"
  #--------------------------------------
  # dingding 默认用户userid
  #   88888 对应钉钉用户名 
  #-----------------------
default_receiver_userid:
  - "88888"

providers:
  # 渠道 1: 钉钉应用私信
  dingtalk_robot:
    app_key: "*****"
    app_secret: "*****"
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

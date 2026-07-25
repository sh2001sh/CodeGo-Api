package app

import (
	"bytes"
	"html/template"
	"strings"
)

type transactionalEmailData struct {
	SystemName  string
	Title       string
	Message     string
	Code        string
	ActionLabel string
	ActionURL   string
	FallbackURL string
	Validity    int
}

var transactionalEmailTemplate = template.Must(template.New("transactional-email").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body style="margin:0;padding:0;background:#eef2f4;color:#17212b;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','Microsoft YaHei',sans-serif;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="background:#eef2f4;">
    <tr><td align="center" style="padding:32px 16px;">
      <table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="max-width:600px;background:#ffffff;border-radius:8px;overflow:hidden;">
        <tr><td style="padding:28px 32px 24px;background:#0f766e;color:#ffffff;">
          <div style="font-size:24px;font-weight:700;line-height:1.25;">{{.SystemName}}</div>
          <div style="margin-top:6px;font-size:14px;line-height:1.5;color:#d8f3ed;">账户安全通知</div>
        </td></tr>
        <tr><td style="padding:32px;">
          <h1 style="margin:0 0 16px;font-size:22px;line-height:1.35;color:#17212b;">{{.Title}}</h1>
          <p style="margin:0;font-size:15px;line-height:1.75;color:#455461;">{{.Message}}</p>
          {{if .Code}}
          <div style="margin:24px 0;padding:18px 16px;background:#f5f8f8;border:1px solid #d8e5e2;border-radius:6px;text-align:center;">
            <div style="font-size:12px;line-height:1.5;color:#64727c;">验证码</div>
            <div style="margin-top:6px;font-family:ui-monospace,SFMono-Regular,Consolas,monospace;font-size:30px;font-weight:700;letter-spacing:6px;color:#0f766e;">{{.Code}}</div>
          </div>
          {{end}}
          {{if .ActionURL}}
          <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin:24px 0;"><tr><td style="border-radius:6px;background:#0f766e;">
            <a href="{{.ActionURL}}" style="display:inline-block;padding:12px 20px;color:#ffffff;text-decoration:none;font-size:15px;font-weight:600;">{{.ActionLabel}}</a>
          </td></tr></table>
          {{end}}
          <p style="margin:0;font-size:13px;line-height:1.7;color:#64727c;">此操作链接或验证码将在 {{.Validity}} 分钟后失效。如非本人操作，请忽略此邮件。</p>
          {{if .FallbackURL}}
          <p style="margin:20px 0 0;font-size:12px;line-height:1.7;color:#64727c;word-break:break-all;">按钮无法打开？请复制此链接：<br><a href="{{.FallbackURL}}" style="color:#0f766e;text-decoration:underline;">{{.FallbackURL}}</a></p>
          {{end}}
        </td></tr>
        <tr><td style="padding:18px 32px;background:#f5f8f8;font-size:12px;line-height:1.6;color:#7b8790;">此邮件由 {{.SystemName}} 自动发送，请勿直接回复。</td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`))

func renderRegistrationVerificationEmail(systemName, code string, validity int) string {
	return renderTransactionalEmail(transactionalEmailData{
		SystemName: systemName,
		Title:      "验证你的邮箱",
		Message:    "你正在注册或绑定邮箱，请在页面中输入以下验证码完成验证。",
		Code:       code,
		Validity:   validity,
	})
}

func renderPasswordResetEmail(systemName, link string, validity int) string {
	return renderTransactionalEmail(transactionalEmailData{
		SystemName:  systemName,
		Title:       "重置你的密码",
		Message:     "我们收到了重置密码的请求。请通过下方按钮继续操作。",
		ActionLabel: "重置密码",
		ActionURL:   link,
		FallbackURL: link,
		Validity:    validity,
	})
}

func renderTransactionalEmail(data transactionalEmailData) string {
	data.SystemName = strings.TrimSpace(data.SystemName)
	if data.SystemName == "" {
		data.SystemName = "codego-api"
	}

	var content bytes.Buffer
	if err := transactionalEmailTemplate.Execute(&content, data); err != nil {
		return ""
	}
	return content.String()
}

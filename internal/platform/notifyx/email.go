package notifyx

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"net/smtp"
	"slices"
	"strings"
	"time"

	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
)

// SendEmail sends an HTML email using the configured SMTP transport.
func SendEmail(subject string, receiver string, content string) error {
	smtpFrom := platformconfig.SMTPFrom
	if smtpFrom == "" {
		smtpFrom = platformconfig.SMTPAccount
	}

	id, err := generateMessageID(smtpFrom)
	if err != nil {
		return err
	}
	if platformconfig.SMTPServer == "" && platformconfig.SMTPAccount == "" {
		return fmt.Errorf("SMTP 服务器未配置")
	}

	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	mail := []byte(fmt.Sprintf(
		"To: %s\r\nFrom: %s <%s>\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n",
		receiver,
		platformconfig.SystemName,
		smtpFrom,
		encodedSubject,
		time.Now().Format(time.RFC1123Z),
		id,
		content,
	))
	auth := getSMTPAuth()
	addr := fmt.Sprintf("%s:%d", platformconfig.SMTPServer, platformconfig.SMTPPort)
	to := strings.Split(receiver, ";")

	if platformconfig.SMTPPort == 465 || platformconfig.SMTPSSLEnabled {
		if err := sendSMTPTLSEmail(auth, smtpFrom, receiver, mail); err != nil {
			platformobservability.SysError(fmt.Sprintf("failed to send email to %s: %v", receiver, err))
			return err
		}
		return nil
	}

	err = smtp.SendMail(addr, auth, smtpFrom, to, mail)
	if err != nil {
		platformobservability.SysError(fmt.Sprintf("failed to send email to %s: %v", receiver, err))
	}
	return err
}

func generateMessageID(smtpFrom string) (string, error) {
	split := strings.Split(smtpFrom, "@")
	if len(split) < 2 {
		return "", fmt.Errorf("invalid SMTP account")
	}
	domain := split[1]
	randomPart := platformruntime.GetRandomString(12)
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), randomPart, domain), nil
}

func shouldUseSMTPLoginAuth() bool {
	if platformconfig.SMTPForceAuthLogin {
		return true
	}
	return isOutlookServer(platformconfig.SMTPAccount) || slices.Contains(platformconfig.EmailLoginAuthServerList, platformconfig.SMTPServer)
}

func getSMTPAuth() smtp.Auth {
	if shouldUseSMTPLoginAuth() {
		return loginAuth(platformconfig.SMTPAccount, platformconfig.SMTPToken)
	}
	return smtp.PlainAuth("", platformconfig.SMTPAccount, platformconfig.SMTPToken, platformconfig.SMTPServer)
}

func sendSMTPTLSEmail(auth smtp.Auth, smtpFrom string, receiver string, mail []byte) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         platformconfig.SMTPServer,
	}
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", platformconfig.SMTPServer, platformconfig.SMTPPort), tlsConfig)
	if err != nil {
		return err
	}

	client, err := smtp.NewClient(conn, platformconfig.SMTPServer)
	if err != nil {
		return err
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return err
	}
	if err = client.Mail(smtpFrom); err != nil {
		return err
	}
	for _, target := range strings.Split(receiver, ";") {
		if err = client.Rcpt(target); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(mail); err != nil {
		return err
	}
	return w.Close()
}

type outlookAuth struct {
	username string
	password string
}

func loginAuth(username string, password string) smtp.Auth {
	return &outlookAuth{username: username, password: password}
}

func (a *outlookAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

func (a *outlookAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch string(fromServer) {
	case "Username:":
		return []byte(a.username), nil
	case "Password:":
		return []byte(a.password), nil
	default:
		return nil, errors.New("unknown fromServer")
	}
}

func isOutlookServer(server string) bool {
	return strings.Contains(server, "outlook") || strings.Contains(server, "onmicrosoft")
}

package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/jordan-wright/email"
	"go.uber.org/zap"
	"jaytaylor.com/html2text"
)

var sugar *zap.SugaredLogger
var user string
var passwd string
var webhook *template.Template

// The Backend implements SMTP server methods.
type Backend struct{}

// NewSession is called after client greeting (EHLO, HELO).
func (bkd *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	sugar.Debug("New Connect: ", c.Conn().RemoteAddr().String())
	return &Session{}, nil
}

// A Session is returned after successful login.
type Session struct {
}

// AuthPlain implements authentication using SASL PLAIN.
func (s *Session) AuthPlain(username, password string) error {
	if username != user || password != passwd {
		sugar.Error("invalid username or password")
		return errors.New("invalid username or password")
	}

	return nil
}

func (s *Session) Mail(from string, _ *smtp.MailOptions) error {
	sugar.Debug("Mail from:", from)
	return nil
}

func (s *Session) Rcpt(to string, _ *smtp.RcptOptions) error {
	sugar.Debug("Rcpt to:", to)
	return nil
}

func (s *Session) Data(r io.Reader) (err error) {
	title, content, err := ReadData(r)
	if err != nil {
		sugar.Error("ReadData error ", err)
		return err
	}

	err = CallWebhook(title, content)
	return
}

func (s *Session) Reset() {}

func (s *Session) Logout() error { return nil }

func ReadData(r io.Reader) (title, content string, err error) {
	e, err := email.NewEmailFromReader(r)
	if err != nil {
		return "", "", err
	}
	title = e.Subject
	if e.Text != nil {
		content = string(e.Text)
	} else {
		content, err = html2text.FromString(string(e.HTML), html2text.Options{PrettyTables: true})
		if err != nil {
			return "", "", err
		}
	}

	sugar.Debug("Data:", content)
	return
}

func CallWebhook(title, content string) (err error) {
	var urlSb strings.Builder
	err = webhook.Execute(&urlSb, map[string]string{
		"title":   url.QueryEscape(title),
		"content": url.QueryEscape(content),
	})
	if err != nil {
		return err
	}
	urlAddr := urlSb.String()

	var resp *http.Response

	resp, err = http.Get(urlAddr)

	if err != nil {
		sugar.Error("Call Webhook error! ", err)
	} else {
		sugar.Info("Call Webhook success:", resp)
	}
	return
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalln(err)
	}
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {
			log.Fatalln(err)
		}
	}(logger)
	sugar = logger.Sugar()

	user = os.Getenv("USERNAME")
	passwd = os.Getenv("PASSWORD")
	webhook, err = template.New("url").Parse(os.Getenv("WEBHOOK"))
	if err != nil {
		sugar.Error("webhook template create error ", err)
		os.Exit(1)
	}

	be := &Backend{}
	s := smtp.NewServer(be)
	s.Addr = "0.0.0.0:8587"
	s.Domain = "fevenor.com"
	s.WriteTimeout = 10 * time.Second
	s.ReadTimeout = 10 * time.Second
	s.MaxMessageBytes = 1024 * 1024
	s.MaxRecipients = 50
	s.AllowInsecureAuth = true
	sugar.Info("Starting server at ", s.Addr)
	if err := s.ListenAndServe(); err != nil {
		sugar.Error("SMTP server start error ", err)
		os.Exit(1)
	}
}

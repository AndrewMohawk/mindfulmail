package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"

	"time"

	"github.com/emersion/go-smtp"
	"github.com/jhillyerd/enmime"
	openai "github.com/sashabaranov/go-openai"
)

// The Backend implements SMTP server methods.
type Backend struct{}

func (bkd *Backend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &Session{}, nil
}

// A Session is returned after EHLO.
type Session struct{}

func (s *Session) AuthPlain(username, password string) error {

	return nil
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	log.Println("Mail from:", from)
	return nil
}

func (s *Session) Rcpt(to string) error {
	log.Println("Rcpt to:", to)
	return nil
}

func sendEmail(to, subject, body, from string) error {

	// Setup an unencrypted connection to a local mail server.
	c, err := smtp.Dial("localhost:26")
	if err != nil {
		return err
	}
	defer c.Close()

	msg := strings.NewReader("To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")
	toArr := []string{to}

	err = c.SendMail(from, toArr, msg)
	if err != nil {
		log.Fatal(err)
		return err
	}

	fmt.Println("Email sent successfully")

	return nil
}

// function to summarise the text with OpenAI
func summariseText(text string) string {
	client := openai.NewClient("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")
	tokenPrefix := "This is a service to help people avoid harmful content in emails. Emails are sent to this service and you are here to help people remove the harmful and offensive content and give a summary of the email. Please summarise the email below:"
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleUser,
					Content: tokenPrefix + `
					` + text + `
					`,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return "ChatCompletion error: %v\n" + err.Error()
	}

	return resp.Choices[0].Message.Content
}

func (s *Session) Data(ioReaderEmail io.Reader) error {
	// NOTE: for some reason, if we directly pass r to enmime, result is not correct
	data, err := ioutil.ReadAll(ioReaderEmail)
	if err != nil {
		return err
	}

	env, err := enmime.ReadEnvelope(bytes.NewReader(data))
	if err != nil {
		return err
	}

	body := env.Text

	summary := summariseText(body)
	sendEmail(env.GetHeader("From"), "Summarised Email: "+env.GetHeader("Subject"), summary, "summariser@mindfulmail.net")

	return nil
}

func (s *Session) Reset() {}

func (s *Session) Logout() error {
	return nil
}

func main() {
	be := &Backend{}

	s := smtp.NewServer(be)

	s.Addr = ":25"
	s.Domain = "localhost"
	s.ReadTimeout = 10 * time.Second
	s.WriteTimeout = 10 * time.Second
	s.MaxMessageBytes = 1024 * 1024
	s.MaxRecipients = 50
	s.AllowInsecureAuth = true

	log.Println("Starting server at", s.Addr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

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

	"database/sql"

	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
)

// The Backend implements SMTP server methods.
type Backend struct{}

type message struct {
	From string
	To   []string
	Data []byte
}

// A Session is returned after EHLO.
type session struct {
	msg *message
}

func (bkd *Backend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &session{}, nil
}

func (s *session) Reset() {
	s.msg = &message{}
}

func (s *session) AuthPlain(username, password string) error {

	return nil
}

func (s *session) Mail(from string, opts *smtp.MailOptions) error {
	log.Println("Mail from:", from)
	s.Reset()
	s.msg.From = from

	return nil
}

func (s *session) Rcpt(to string) error {
	log.Println("Rcpt to:", to)
	s.msg.To = append(s.msg.To, to)
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

func (s *session) Data(ioReaderEmail io.Reader) error {
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
	from := s.msg.From

	subject := env.GetHeader("Subject")

	dbPath := "/home/mindfulmail/database/mindfulmail.db"
	//dbPath := "./database/mindfulmail.db"
	db, err := sql.Open("sqlite3", dbPath)
	fmt.Println(s.msg.From)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	// Lets make sure the from user (from) is in the database
	var fromID int
	err = db.QueryRow("SELECT id FROM users WHERE email = ?", from).Scan(&fromID)
	if err != nil {
		// Lets print the query to the console
		fmt.Println("SELECT id FROM users WHERE email = ?", from)
		fmt.Println(err.Error())
		fmt.Println("User not found in database, ignoring email, current valid users:")
		// Lets show all the users
		rows, err := db.Query("SELECT id, email FROM users")
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var id int
			var email string
			err = rows.Scan(&id, &email)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(id, email)
		}
		return nil
	}
	//fmt.Println("User found in database, id:", fromID)

	summary := summariseText(body)
	sendEmail(from, "Summarised Email: "+subject, summary, "help@mindfulmail.net")

	return nil
}

func (s *session) Logout() error {
	return nil
}

func main() {
	// Variable to store the database path

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

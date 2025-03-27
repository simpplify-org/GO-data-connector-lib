package email

import (
	"bytes"
	"fmt"
	"path"
	"text/template"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type SmtpConfig struct {
	Email    string
	Password string
	Host     string
	Port     string
	Sender   string
}

type Sendgrid struct {
	AssetsDirectory string
	SMTP            SmtpConfig
	Client          *sendgrid.Client
}

func NewSendgrid(assetsDirectory string, SMTP SmtpConfig, sendgridApiKey string) *Sendgrid {
	client := sendgrid.NewSendClient(sendgridApiKey)
	return &Sendgrid{
		AssetsDirectory: assetsDirectory,
		SMTP:            SMTP,
		Client:          client,
	}
}

func (s *Sendgrid) NewTemplate(
	placeHolder any,
	templateHtml string,
) (*string, error) {
	var w string

	filePath := path.Join(s.AssetsDirectory, templateHtml)

	tmpl, err := template.ParseFiles(filePath)
	if err != nil {
		return nil, err
	}

	var tpl bytes.Buffer
	if err := tmpl.Execute(&tpl, placeHolder); err != nil {
		return nil, err
	}
	w = tpl.String()
	return &w, nil
}

func (s *Sendgrid) Send(
	template, email, title, receiver, plainTextContent string,
) error {
	from := mail.NewEmail(s.SMTP.Sender, s.SMTP.Email)

	subject := title

	to := mail.NewEmail(receiver, email)

	htmlContent := template

	message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlContent)

	response, err := s.Client.Send(message)
	if err != nil {
		return fmt.Errorf("error: failed to send email: %v", err)
	}

	if response == nil {
		return fmt.Errorf("error: email delivery failed, no response received")
	}

	if response.StatusCode >= 400 {
		return fmt.Errorf("error: email delivery failed, status code: %d", response.StatusCode)
	}

	return nil
}

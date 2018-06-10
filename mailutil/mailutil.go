package mailutil

import (
	gomail "gopkg.in/gomail.v2"
)

type MailerConfig struct {
	Host     string
	Port     int
	User     string
	Password string
}

type MailMessenger struct {
	mailerConfig MailerConfig
}

func NewMailMessenger(mailerConfig MailerConfig) *MailMessenger {
	return &MailMessenger{
		mailerConfig: mailerConfig,
	}
}

func (m *MailMessenger) Send(msg *Message) error {
	var d *gomail.Dialer

	d = gomail.NewDialer(
		m.mailerConfig.Host,
		m.mailerConfig.Port,
		m.mailerConfig.User,
		m.mailerConfig.Password,
	)

	goMessage := gomail.NewMessage()
	goMessage.SetHeaders(msg.GetHeaders())
	goMessage.SetBody("text/html", msg.GetMessage())

	imgUrls := msg.GetImages()
	for _, imagePath := range imgUrls {
		goMessage.Embed(imagePath)
	}

	return d.DialAndSend(goMessage)
}

type SendMessage interface {
	Send(msg *Message) error
}

type Message struct {
	headers       map[string][]string
	message       string
	messageFormat string
	images        []string
}

func (m *Message) SetEmbedImages(images ...string) {
	m.images = images
}

func (m *Message) SetHeaders(headers map[string][]string) {
	m.headers = headers
}

func (m *Message) SetMessage(message string) {
	m.message = message
}

func (m *Message) SetMessageFormat(format string) {
	m.messageFormat = format
}

func (m *Message) GetHeaders() map[string][]string {
	return m.headers
}

func (m *Message) GetMessage() string {
	return m.message
}

func (m *Message) GetMessageFormat() string {
	return m.messageFormat
}

func (m *Message) GetImages() []string {
	return m.images
}

func SendEmail(
	to []string,
	from string,
	subject string,
	imgUrls []string,
	template []byte,
	messenger SendMessage,
) error {
	m := &Message{}
	m.SetHeaders(map[string][]string{
		"From":    []string{from},
		"To":      to,
		"Subject": []string{subject},
	})
	m.SetMessage(string(template))

	if imgUrls != nil {
		m.SetEmbedImages(imgUrls...)
	}

	m.SetMessageFormat("text/html")
	return messenger.Send(m)
}

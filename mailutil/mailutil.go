package mailutil

import (
	gomail "gopkg.in/gomail.v2"
)

// MailerConfig is config struct that enables user to set up mailing
// This config struct is used in the NewMailMessenger function
type MailerConfig struct {
	// Host is the host to connect to send message
	Host string
	// Port is the port to connect to to send message
	Port int
	// User is the user to use as authentication to send message
	User string
	// Password is the password to use for authntaication to send message
	Password string
}

// MailMessenger sends mails based on mailerconfig
type MailMessenger struct {
	mailerConfig MailerConfig
}

// NewMailMessenger returns *MailMessenger based on the mailerconfig passed
func NewMailMessenger(mailerConfig MailerConfig) *MailMessenger {
	return &MailMessenger{
		mailerConfig: mailerConfig,
	}
}

// Send sends email based on msg config passed
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

// SendMessage is meant to be used to send some type of message
type SendMessage interface {
	Send(msg *Message) error
}

// Message is what will be sent to the client including
// headers, embeded images and a message itself
type Message struct {
	headers       map[string][]string
	message       string
	messageFormat string
	images        []string
}

// SetEmbedImages takes a like of file names and embeds them into
// the message
func (m *Message) SetEmbedImages(images ...string) {
	m.images = images
}

// SetHeaders sets the message's header
func (m *Message) SetHeaders(headers map[string][]string) {
	m.headers = headers
}

// SetMessage sets the message that will be sent
func (m *Message) SetMessage(message string) {
	m.message = message
}

// SetMessageFormat sets the message format eg. "html/text"
func (m *Message) SetMessageFormat(format string) {
	m.messageFormat = format
}

// GetHeaders returns the current message's headers
func (m *Message) GetHeaders() map[string][]string {
	return m.headers
}

// GetMessage returns the current message
func (m *Message) GetMessage() string {
	return m.message
}

// GetMessageFormat returns the current message's format
func (m *Message) GetMessageFormat() string {
	return m.messageFormat
}

// GetImages returns the current message's embeded images
func (m *Message) GetImages() []string {
	return m.images
}

// SendEmail is generic shortcut method for sending an email
// to: slice of string emails to send message
// from: Email address that you will be sending email from
// subject: Subject of the email
// imgUrls: Slice of strings of image files that you wish to embed in message
// template: Template that will be sent in email.  This parameter will most
// likely come from template.#Template.Execute function
// messenger: Sends the email
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

package mailer

import (
	"fmt"
	"net/smtp"
)

// Mailer sluzi za slanje mejlova preko MailHog SMTP servera (port 1025)
// Poslati mejlovi se vide u browseru na http://localhost:8025
type Mailer struct {
	host string
	port string
	from string
}

func New(host, port, from string) *Mailer {
	return &Mailer{
		host: host,
		port: port,
		from: from,
	}
}

func (m *Mailer) SendEmail(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%s", m.host, m.port)
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"UTF-8\"\r\n\r\n%s\r\n",
		m.from, to, subject, body,
	)
	return smtp.SendMail(addr, nil, m.from, []string{to}, []byte(msg))
}

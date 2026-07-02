package chat

import (
	"log"
	"regexp"

	appsvc "github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/config"
	"github.com/SteVio89/stevio-home/mailer"
)

type ChatHandler struct {
	app    *appsvc.App
	log    *log.Logger
	cfg    *config.Config
	mailer *mailer.Mailer
}

func New(app *appsvc.App, logger *log.Logger, cfg *config.Config, mail *mailer.Mailer) *ChatHandler {
	return &ChatHandler{app: app, log: logger, cfg: cfg, mailer: mail}
}

var emailRegex = regexp.MustCompile(`\S+@\S+\.\S+`)

func sanitizeMessageBody(body string) string {
	return emailRegex.ReplaceAllString(body, "[email removed -- please use the Share Email button]")
}

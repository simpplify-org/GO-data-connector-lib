package slack

import (
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/slack-go/slack"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

type Config struct {
	SlackToken       string
	ChannelID        string
	OnlyPanics       bool
	Debug            bool
	JSONErrorField   string
	JSONMessageField string
}

func NewSlackConfig(SlackToken, ChannelID string) *Config {
	return &Config{SlackToken: SlackToken, ChannelID: ChannelID}
}

type Reporter struct {
	config Config
	client *slack.Client
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       []byte
	jsonError  string
}

type ErrorResponse struct {
	Error   interface{} `json:"error,omitempty"`
	Message interface{} `json:"message,omitempty"`
}

func New(config Config) *Reporter {
	if config.SlackToken == "" {
		log.Fatal("SlackToken required")
	}
	if config.ChannelID == "" {
		log.Fatal("ChannelID required")
	}
	return &Reporter{
		config: config,
		client: slack.New(config.SlackToken),
	}
}

func (r *Reporter) WrapHandler(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		recorder := &responseRecorder{ResponseWriter: w}
		w = recorder
		path := req.URL.Path
		method := req.Method

		defer func() {
			if recovered := recover(); recovered != nil {
				r.HandlePanic(recovered, path, method)
				panic(recovered)
			}

		}()

		next.ServeHTTP(recorder, req)

		if r.config.OnlyPanics {
			return
		}

		if recorder.statusCode >= 400 {
			r.HandleError(recorder, path, method)
		}

	})
}

func (r *Reporter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			handler := r.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				recorder := &responseRecorder{ResponseWriter: w}
				c.Response().Writer = recorder
				c.SetRequest(req)
				c.Response().Writer = w
				next(c)
			}))
			handler.ServeHTTP(c.Response(), c.Request())
			return nil
		}
	}
}

func (r *Reporter) HandlePanic(recovered interface{}, path string, method string) {
	stack := string(debug.Stack())
	message := fmt.Sprintf(
		"*PANIC CAPTURADO ☠ *\n"+
			"*Rota:* `%s`\n"+
			"*Método:* `%s`\n"+
			"*Erro:* `%v`\n"+
			"*Hora:* `%v`\n"+
			"*Stack:* ```%s```",
		path,
		method,
		recovered,
		time.Now().Format(time.RFC3339),
		stack,
	)
	r.sendToSlack(message)
}

func (r *Reporter) HandleError(recorder *responseRecorder, path string, method string) {
	var errorMsg string
	if recorder.jsonError != "" {
		errorMsg = recorder.jsonError
	} else {
		errorMsg = string(recorder.body)
		if errorMsg == "" {
			errorMsg = http.StatusText(recorder.statusCode)
		}
	}

	stack := string(debug.Stack())
	message := fmt.Sprintf(
		"*⚠️ ERRO CAPTURADO*\n"+
			"• *Rota:* `%s`\n"+
			"• *Método:* `%s`\n"+
			"• *Status:* %d\n"+
			"• *Erro:* ```%s```\n"+
			"• *Stack:* ```%s```",
		path, method, recorder.statusCode, errorMsg, stack)

	r.sendToSlack(message)
}

func (r *Reporter) sendToSlack(message string) error {
	if r.config.Debug {
		log.Println("Debug mode - Slack message:", message)
		return nil
	}

	_, _, err := r.client.PostMessage(
		r.config.ChannelID,
		slack.MsgOptionText(message, false),
		slack.MsgOptionEnableLinkUnfurl(),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		log.Printf("Error sending to Slack: %v\n", err)
		return err
	}
	return nil
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.statusCode >= 400 {
		r.body = make([]byte, len(p))
		copy(r.body, p)

		var errResp ErrorResponse
		if err := json.Unmarshal(p, &errResp); err == nil {
			if errResp.Error != nil {
				r.jsonError = fmt.Sprintf("%v", errResp.Error)
			} else if errResp.Message != nil {
				r.jsonError = fmt.Sprintf("%v", errResp.Message)
			}
		}
	}
	return r.ResponseWriter.Write(p)
}

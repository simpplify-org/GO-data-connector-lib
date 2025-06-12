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
	SlackToken string
	ChannelID  string
	OnlyPanics bool
	Debug      bool
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

func (r *Reporter) EchoMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			recorder := &responseRecorder{ResponseWriter: c.Response().Writer}
			c.Response().Writer = recorder
			path := c.Request().URL.Path
			method := c.Request().Method

			defer func() {
				if recovered := recover(); recovered != nil {
					r.HandlePanic(recovered, path, method)
					panic(recovered)
				}
			}()

			err := next(c)

			if r.config.OnlyPanics {
				return err
			}

			if err != nil {
				httpErr, ok := err.(*echo.HTTPError)
				if !ok {
					httpErr = echo.NewHTTPError(http.StatusInternalServerError, err.Error())
				}

				if httpErr.Code == http.StatusNotFound {
					return err
				}

				if httpErr.Message == "" {
					httpErr.Message = http.StatusText(httpErr.Code)
				}

				stack := string(debug.Stack())
				message := fmt.Sprintf(
					"*⚠️ ERRO CAPTURADO*\n"+
						"• *Route:* `%s`\n"+
						"• *Method:* `%s`\n"+
						"• *Status:* %d %s\n"+
						"• *Error:* ```%s```\n"+
						"• *Hora:* `%v`\n"+
						"• *Stack:* ```%s```",
					path, method, httpErr.Code, http.StatusText(httpErr.Code), httpErr.Message, time.Now().Format(time.RFC3339), stack)

				r.SendToSlack(message)
				return err
			}

			if recorder.statusCode >= 400 {
				errorMsg := string(recorder.body)
				if errorMsg == "" {
					errorMsg = http.StatusText(recorder.statusCode)
				}

				stack := string(debug.Stack())
				message := fmt.Sprintf(
					"*⚠️ ERRO CAPTURADO*\n"+
						"• *Route:* `%s`\n"+
						"• *Method:* `%s`\n"+
						"• *Status:* %d\n"+
						"• *Error:* ```%s```\n"+
						"• *Hour:* `%v`\n"+
						"• *Stack:* ```%s```",
					path, method, recorder.statusCode, errorMsg, time.Now().Format(time.RFC3339), stack)

				r.SendToSlack(message)
			}

			return err
		}
	}
}

func (r *Reporter) HandlePanic(recovered interface{}, path string, method string) {
	stack := string(debug.Stack())
	message := fmt.Sprintf(
		"*PANIC CAPTURADO ☠ *\n"+
			"*Route:* `%s`\n"+
			"*Method:* `%s`\n"+
			"*Status:* `%v`\n"+
			"*Hour:* `%v`\n"+
			"*Stack:* ```%s```",
		path,
		method,
		recovered,
		time.Now().Format(time.RFC3339),
		stack,
	)
	r.SendToSlack(message)
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

	if recorder.statusCode == http.StatusNotFound {
		return
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

	r.SendToSlack(message)
}

func (r *Reporter) SendToSlack(message string) error {
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
	if r.statusCode >= 400 && r.statusCode != 404 {
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

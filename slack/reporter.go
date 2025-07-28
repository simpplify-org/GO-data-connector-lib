package slack

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/slack-go/slack"
)

type Config struct {
	SlackToken        string
	ChannelID         string
	CriticalChannelID string // Novo campo opcional
	OnlyPanics        bool
	Debug             bool
	Timeout           time.Duration // Novo campo opcional
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
	if config.CriticalChannelID == "" {
		log.Fatal("CriticalChannelID required")
	}

	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	return &Reporter{
		config: config,
		client: slack.New(config.SlackToken, slack.OptionHTTPClient(&http.Client{
			Timeout: config.Timeout,
		})),
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

				// Tratamento especial para erros 502
				if httpErr.Code == http.StatusBadGateway {
					errorMsg := fmt.Sprintf("%v", httpErr.Message)
					if strings.Contains(strings.ToLower(errorMsg), "token is invalid") && r.config.CriticalChannelID != "" {
						r.sendToChannel(r.config.CriticalChannelID, createTokenErrorMessage(c, errorMsg))
						return err
					}

					if strings.Contains(strings.ToLower(errorMsg), "token has expired") && r.config.CriticalChannelID != "" {
						r.sendToChannel(r.config.CriticalChannelID, createTokenErrorMessage(c, errorMsg))
						return err
					}
				}

				if httpErr.Code == http.StatusNotFound {
					return err
				}

				if httpErr.Message == "" {
					httpErr.Message = http.StatusText(httpErr.Code)
				}

				r.SendToSlack(createErrorMessage(path, method, httpErr.Code, fmt.Sprintf("%v", httpErr.Message)))
				return err
			}

			if recorder.statusCode >= 400 {
				r.HandleError(recorder, path, method)
			}

			return err
		}
	}
}

func (r *Reporter) HandlePanic(recovered interface{}, path string, method string) {
	message := fmt.Sprintf(
		"*PANIC CAPTURADO ☠ *\n"+
			"*Route:* `%s`\n"+
			"*Method:* `%s`\n"+
			"*Status:* `%v`\n"+
			"*Hour:* `%v`\n",
		path,
		method,
		recovered,
		time.Now().Format(time.RFC3339),
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

	// Tratamento especial para erros 502
	if recorder.statusCode == http.StatusBadGateway {
		if strings.Contains(strings.ToLower(errorMsg), "token is invalid") && r.config.CriticalChannelID != "" {
			r.sendToChannel(r.config.CriticalChannelID, createTokenErrorMessageFromRecorder(path, method, recorder.statusCode, errorMsg))
			return
		}
	}

	r.SendToSlack(createErrorMessage(path, method, recorder.statusCode, errorMsg))
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

// Novas funções auxiliares
func (r *Reporter) sendToChannel(channelID, message string) error {
	if r.config.Debug {
		log.Printf("[DEBUG] Would send to channel %s: %s", channelID, message)
		return nil
	}

	_, _, err := r.client.PostMessage(
		channelID,
		slack.MsgOptionText(message, false),
		slack.MsgOptionEnableLinkUnfurl(),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		log.Printf("Error sending to Slack channel %s: %v", channelID, err)
		return err
	}
	return nil
}

func createTokenErrorMessage(c echo.Context, errorMsg string) string {
	return fmt.Sprintf(
		"🚨 *ERRO CRÍTICO - TOKEN INVÁLIDO* 🚨\n"+
			"• *URL:* `%s`\n"+
			"• *Método:* `%s`\n"+
			"• *Erro:* ```%s```\n"+
			"• *Hora:* %s\n"+
			"• *IP:* %s",
		c.Path(),
		c.Request().Method,
		errorMsg,
		time.Now().Format(time.RFC3339),
		c.RealIP())
}

func createTokenErrorMessageFromRecorder(path, method string, statusCode int, errorMsg string) string {
	return fmt.Sprintf(
		"🚨 *ERRO CRÍTICO - TOKEN INVÁLIDO* 🚨\n"+
			"• *Rota:* `%s`\n"+
			"• *Método:* `%s`\n"+
			"• *Status:* %d %s\n"+
			"• *Erro:* ```%s```\n"+
			"• *Hora:* %s",
		path,
		method,
		statusCode,
		http.StatusText(statusCode),
		errorMsg,
		time.Now().Format(time.RFC3339))
}

func createErrorMessage(path, method string, statusCode int, errorMsg string) string {
	return fmt.Sprintf(
		"*⚠️ ERRO CAPTURADO*\n"+
			"• *Rota:* `%s`\n"+
			"• *Método:* `%s`\n"+
			"• *Status:* %d %s\n"+
			"• *Erro:* ```%s```\n"+
			"• *Hora:* %s",
		path,
		method,
		statusCode,
		http.StatusText(statusCode),
		errorMsg,
		time.Now().Format(time.RFC3339))
}

func (r *Reporter) SendImageToSlack(filePath, title string) error {
	return r.SendImageToSpecificChannel(r.config.ChannelID, filePath, title)
}

// SendImageToSpecificChannel envia uma imagem para um canal específico do Slack
func (r *Reporter) SendImageToSpecificChannel(channelID, filePath, title string) error {
	if r.config.Debug {
		log.Printf("[DEBUG] Enviaria imagem para canal %s: %s", channelID, filePath)
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Erro ao abrir arquivo %s: %v", filePath, err)
		return err
	}
	defer file.Close()

	// Usa FileInfo para obter tamanho e nome
	info, err := file.Stat()
	if err != nil {
		log.Printf("Erro ao obter info do arquivo %s: %v", filePath, err)
		return err
	}

	params := slack.UploadFileV2Parameters{
		Reader:         file,
		Filename:       info.Name(),
		FileSize:       int(info.Size()),
		Title:          title,
		Channel:        channelID,
		InitialComment: "QR Code gerado para autenticação do WhatsApp",
	}

	_, err = r.client.UploadFileV2(params)
	if err != nil {
		log.Printf("Erro ao enviar imagem para Slack: %v", err)
	}
	return err
}

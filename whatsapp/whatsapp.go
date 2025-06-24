package whatsapp

import (
	"github.com/twilio/twilio-go"
	openApi "github.com/twilio/twilio-go/rest/api/v2010"
	"log"
)

type TwilioConfig struct {
	AccountSID string
	AuthToken  string
	From       string
}

type TwilioApi struct {
	Config TwilioConfig
	Client *twilio.RestClient
}

func New(config TwilioConfig) *TwilioApi {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: config.AccountSID,
		Password: config.AuthToken,
	})

	if config.AccountSID == "" {
		log.Fatalln("Account SID not set")
	}

	if config.AuthToken == "" {
		log.Fatalln("AuthToken not set")
	}

	return &TwilioApi{
		Config: config,
		Client: client,
	}
}

func (api *TwilioApi) SendMessage(toNumber, message string) error {
	params := &openApi.CreateMessageParams{}
	params.SetFrom(api.Config.From)
	params.SetTo(toNumber)
	params.SetBody(message)

	resp, err := api.Client.Api.CreateMessage(params)
	if err != nil {
		log.Fatal(err)
	} else {
		log.Println(*resp.Body)
	}
	return err
}

func (api *TwilioApi) SendManyMessage(toNumbers []string, message string) error {
	numbersErrors := make(map[string]string)
	var err error

	for _, number := range toNumbers {
		err = api.SendMessage(number, message)
		if err != nil {
			numbersErrors[number] = err.Error()
		}
	}
	return err
}

// Package firstaccessinvite concentra o envio do convite de primeiro acesso do cedente (Orbeo)
// pelo AWS SES e as constantes que GO-fidc-service e GO-contact-service combinavam "na mão":
// a tag "kind", os status do convite e o assunto do e-mail.
//
// O pacote não conhece o schema do FIDC. A gravação do convite fica atrás da interface Store,
// que cada serviço implementa com o seu banco (hoje, a tabela convites_primeiro_acesso_email).
//
// O convite pode sair antes de a organização existir: o contact-service manda o e-mail e a
// organização é criada depois, pela fila. Por isso o Record só carrega o CNPJ (Documento), nunca
// um id de organização.
//
// Regra de canal, preservada dos dois serviços: o SES só é usado quando FromEmail está
// preenchido (SES_FROM_EMAIL); sem ele, Deliver cai no Fallback (SendGrid).
package firstaccessinvite

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// Kind é o valor da tag "kind" que identifica o convite nos eventos do SES publicados no SNS.
const Kind = "first_access_invite"

// Nomes das tags que o SES devolve nos eventos.
const (
	TagKind      = "kind"
	TagDocumento = "documento"
)

// Status do convite em convites_primeiro_acesso_email.
const (
	StatusSent         = "ENVIADO"
	StatusDelivered    = "ENTREGUE"
	StatusNotDelivered = "NAO_ENTREGUE"
)

// Subject é o assunto do e-mail do convite, igual para SES e para o fallback.
const Subject = "Bem-vindo à Orbeo - Suas Credenciais de Acesso"

// DefaultReceiverName é o nome de destinatário usado no fallback quando Invite.ReceiverName
// não é informado.
const DefaultReceiverName = "Orbeo"

// ErrNotRecorded indica que o e-mail FOI enviado pelo SES, mas o registro do envio falhou.
// Quem recebe esse erro não deve tentar outro canal (o convite já chegou ao SES): só logar.
var ErrNotRecorded = errors.New("convite enviado, mas o registro do envio falhou")

// ErrNotConfigured é devolvido por Send quando o SES não está configurado.
var ErrNotConfigured = errors.New("SES não configurado")

// Config configura o remetente SES. FromEmail vazio desativa o SES.
type Config struct {
	Region, AccessKey, SecretKey string
	FromEmail, FromName          string
	ConfigurationSet             string
	// ReplyTo é opcional (SES_REPLY_TO): o convite sai de um remetente no-reply, e uma resposta
	// cairia no vazio.
	ReplyTo string
}

// Record é uma linha de convite a gravar. MessageID vazio e Detail vazio viram NULL no banco.
type Record struct {
	Documento string // CNPJ só com dígitos
	Email     string
	MessageID string // MessageId do SES; vazio quando o envio falhou
	Status    string // StatusSent, StatusNotDelivered...
	Detail    string
	Origem    string // quem enviou: "contact", "system", "troca_email"...
}

// Store é a persistência do convite, implementada por quem tem o banco (adapter do FIDC).
type Store interface {
	RecordInvite(ctx context.Context, rec Record) error
}

// Invite é um convite a enviar.
type Invite struct {
	Documento string // CNPJ, com ou sem máscara
	To        string
	HTML      string
	Origem    string
	// ReceiverName só é usado no fallback; vazio vira DefaultReceiverName.
	ReceiverName string
}

// Fallback é o canal usado quando o SES não está configurado. Satisfeita por *email.Sendgrid.
type Fallback interface {
	Send(template, email, title, receiver, plainTextContent string) error
}

// SESClient é o pedaço do cliente SES v2 usado aqui (facilita testes).
type SESClient interface {
	SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

// Sender envia convites pelo SES e registra o resultado no Store.
type Sender struct {
	client SESClient
	config Config
	store  Store
}

// New devolve nil, sem erro, quando cfg.FromEmail está vazio. Os métodos de Sender aceitam
// receiver nil e tratam como SES desativado.
func New(ctx context.Context, cfg Config, store Store) (*Sender, error) {
	if strings.TrimSpace(cfg.FromEmail) == "" {
		return nil, nil
	}
	options := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Region)}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		options = append(options, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("configurar AWS SES: %w", err)
	}
	return NewWithClient(sesv2.NewFromConfig(awsCfg), cfg, store), nil
}

// NewWithClient monta o Sender com um cliente já pronto (testes e injeção).
func NewWithClient(client SESClient, cfg Config, store Store) *Sender {
	return &Sender{client: client, config: cfg, store: store}
}

// Enabled diz se o SES está pronto para enviar.
func (s *Sender) Enabled() bool { return s != nil && s.client != nil }

// Deliver envia o convite. Com SES ativo e CNPJ válido, envia pelo SES (Send); caso contrário
// usa o fallback (SendGrid) com o mesmo assunto. Falha do SES é devolvida ao chamador, que
// decide o próximo canal (ex.: Brevo) e deve chamar RecordAlternative se esse canal enviar.
// Um erro que satisfaz errors.Is(err, ErrNotRecorded) significa e-mail enviado: não reenviar.
func (s *Sender) Deliver(ctx context.Context, inv Invite, fallback Fallback) error {
	if s.Enabled() && OnlyDigits(inv.Documento) != "" {
		return s.Send(ctx, inv)
	}
	if fallback == nil {
		return errors.New("nenhum canal de e-mail configurado para o convite")
	}
	receiver := inv.ReceiverName
	if receiver == "" {
		receiver = DefaultReceiverName
	}
	return fallback.Send(inv.HTML, inv.To, Subject, receiver, Subject)
}

// Send envia o convite pelo SES e registra o resultado. Falha no SES é registrada como
// NAO_ENTREGUE e devolvida (nunca engolida), para o chamador poder usar outro canal.
func (s *Sender) Send(ctx context.Context, inv Invite) error {
	if !s.Enabled() {
		return ErrNotConfigured
	}
	documento := OnlyDigits(inv.Documento)
	out, sendErr := s.client.SendEmail(ctx, s.buildInput(documento, inv.To, inv.HTML))
	if sendErr != nil {
		rec := Record{Documento: documento, Email: inv.To, Status: StatusNotDelivered, Detail: "Falha ao enviar pelo SES: " + sendErr.Error(), Origem: inv.Origem}
		if err := s.record(ctx, rec); err != nil {
			return fmt.Errorf("enviar e-mail pelo SES: %w (registro: %v)", sendErr, err)
		}
		return fmt.Errorf("enviar e-mail pelo SES: %w", sendErr)
	}
	rec := Record{Documento: documento, Email: inv.To, MessageID: aws.ToString(out.MessageId), Status: StatusSent, Origem: inv.Origem}
	if err := s.record(ctx, rec); err != nil {
		return fmt.Errorf("%w: %v", ErrNotRecorded, err)
	}
	return nil
}

// AlternativeRecord é o registro de um convite que saiu por outro canal (Brevo, SendGrid)
// depois de o SES falhar. Sem ele o último registro seria o NAO_ENTREGUE do SES, e o card
// diria "não entregue" de um e-mail que chegou. Não há como confirmar a entrega por esses
// canais.
func AlternativeRecord(documento, to, canal, origem string) Record {
	return Record{Documento: OnlyDigits(documento), Email: to, Status: StatusSent, Detail: "Enviado por " + canal + " (sem confirmação de entrega)", Origem: origem}
}

// RecordAlternative grava AlternativeRecord. Sem SES ativo ou sem CNPJ, não faz nada.
func (s *Sender) RecordAlternative(ctx context.Context, documento, to, canal, origem string) error {
	rec := AlternativeRecord(documento, to, canal, origem)
	if !s.Enabled() || rec.Documento == "" {
		return nil
	}
	return s.record(ctx, rec)
}

func (s *Sender) record(ctx context.Context, rec Record) error {
	if s.store == nil {
		return errors.New("store do convite não configurado")
	}
	return s.store.RecordInvite(ctx, rec)
}

// buildInput monta o SendEmailInput. As tags "kind" e "documento" voltam nos eventos do SES.
func (s *Sender) buildInput(documento, to, html string) *sesv2.SendEmailInput {
	from := s.config.FromEmail
	if s.config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", s.config.FromName, s.config.FromEmail)
	}
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(from),
		Destination:      &types.Destination{ToAddresses: []string{to}},
		Content: &types.EmailContent{Simple: &types.Message{
			Subject: &types.Content{Data: aws.String(Subject), Charset: aws.String("UTF-8")},
			Body:    &types.Body{Html: &types.Content{Data: aws.String(html), Charset: aws.String("UTF-8")}},
		}},
		EmailTags: []types.MessageTag{
			{Name: aws.String(TagKind), Value: aws.String(Kind)},
			{Name: aws.String(TagDocumento), Value: aws.String(documento)},
		},
	}
	if s.config.ReplyTo != "" {
		input.ReplyToAddresses = []string{s.config.ReplyTo}
	}
	if s.config.ConfigurationSet != "" {
		input.ConfigurationSetName = aws.String(s.config.ConfigurationSet)
	}
	return input
}

// OnlyDigits devolve só os dígitos de value (CNPJ sem máscara).
func OnlyDigits(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

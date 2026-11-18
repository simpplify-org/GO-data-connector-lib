package firstaccessinvite

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
)

type fakeSES struct {
	input *sesv2.SendEmailInput
	out   *sesv2.SendEmailOutput
	err   error
}

func (f *fakeSES) SendEmail(_ context.Context, in *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	f.input = in
	return f.out, f.err
}

type fakeStore struct {
	records []Record
	err     error
}

func (f *fakeStore) RecordInvite(_ context.Context, rec Record) error {
	f.records = append(f.records, rec)
	return f.err
}

type fakeFallback struct {
	calls                        int
	template, email, title, name string
	plain                        string
	err                          error
}

func (f *fakeFallback) Send(template, email, title, receiver, plain string) error {
	f.calls++
	f.template, f.email, f.title, f.name, f.plain = template, email, title, receiver, plain
	return f.err
}

func newSender(ses *fakeSES, store *fakeStore) *Sender {
	return NewWithClient(ses, Config{FromEmail: "no-reply@orbeo.site", FromName: "Orbeo", ConfigurationSet: "cfg"}, store)
}

func TestSendBuildsInputAndRecordsSent(t *testing.T) {
	ses := &fakeSES{out: &sesv2.SendEmailOutput{MessageId: aws.String("msg-1")}}
	store := &fakeStore{}
	err := newSender(ses, store).Send(context.Background(), Invite{Documento: "12.345.678/0001-90", To: "a@b.com", HTML: "<p>oi</p>", Origem: "contact"})
	if err != nil {
		t.Fatal(err)
	}
	if got := aws.ToString(ses.input.FromEmailAddress); got != "Orbeo <no-reply@orbeo.site>" {
		t.Errorf("from = %q", got)
	}
	if got := aws.ToString(ses.input.Content.Simple.Subject.Data); got != Subject {
		t.Errorf("subject = %q", got)
	}
	if aws.ToString(ses.input.ConfigurationSetName) != "cfg" {
		t.Error("configuration set não repassado")
	}
	tags := map[string]string{}
	for _, tag := range ses.input.EmailTags {
		tags[aws.ToString(tag.Name)] = aws.ToString(tag.Value)
	}
	if tags[TagKind] != Kind || tags[TagDocumento] != "12345678000190" {
		t.Errorf("tags = %v", tags)
	}
	want := Record{Documento: "12345678000190", Email: "a@b.com", MessageID: "msg-1", Status: StatusSent, Origem: "contact"}
	if len(store.records) != 1 || store.records[0] != want {
		t.Errorf("records = %+v", store.records)
	}
}

func TestSendFailureIsRecordedAndReturned(t *testing.T) {
	sesErr := errors.New("sandbox")
	store := &fakeStore{}
	err := newSender(&fakeSES{err: sesErr}, store).Send(context.Background(), Invite{Documento: "12345678000190", To: "a@b.com", HTML: "x", Origem: "contact"})
	if !errors.Is(err, sesErr) {
		t.Fatalf("erro do SES engolido: %v", err)
	}
	if len(store.records) != 1 || store.records[0].Status != StatusNotDelivered || store.records[0].MessageID != "" {
		t.Errorf("records = %+v", store.records)
	}
}

func TestSendRecordFailureAfterSuccessIsNotASendFailure(t *testing.T) {
	ses := &fakeSES{out: &sesv2.SendEmailOutput{MessageId: aws.String("m")}}
	err := newSender(ses, &fakeStore{err: errors.New("db fora")}).Send(context.Background(), Invite{Documento: "1", To: "a@b.com", HTML: "x"})
	if !errors.Is(err, ErrNotRecorded) {
		t.Fatalf("err = %v, queria ErrNotRecorded", err)
	}
}

func TestDeliverFallsBackWithoutSES(t *testing.T) {
	fb := &fakeFallback{}
	var s *Sender // SES_FROM_EMAIL vazio => New devolve nil
	if err := s.Deliver(context.Background(), Invite{Documento: "123", To: "a@b.com", HTML: "<p>"}, fb); err != nil {
		t.Fatal(err)
	}
	if fb.calls != 1 || fb.title != Subject || fb.plain != Subject || fb.name != DefaultReceiverName || fb.email != "a@b.com" || fb.template != "<p>" {
		t.Errorf("fallback = %+v", fb)
	}
}

func TestDeliverFallsBackWhenDocumentIsEmpty(t *testing.T) {
	ses, fb := &fakeSES{}, &fakeFallback{}
	if err := newSender(ses, &fakeStore{}).Deliver(context.Background(), Invite{Documento: "  ", To: "a@b.com", HTML: "x"}, fb); err != nil {
		t.Fatal(err)
	}
	if ses.input != nil || fb.calls != 1 {
		t.Error("sem CNPJ deveria usar o fallback e não o SES")
	}
}

func TestDeliverUsesSESAndDoesNotFallBackOnFailure(t *testing.T) {
	ses, fb := &fakeSES{err: errors.New("boom")}, &fakeFallback{}
	if err := newSender(ses, &fakeStore{}).Deliver(context.Background(), Invite{Documento: "123", To: "a@b.com", HTML: "x"}, fb); err == nil {
		t.Fatal("falha do SES deveria ser devolvida")
	}
	if fb.calls != 0 {
		t.Error("Deliver não deve trocar de canal sozinho; quem chama decide")
	}
}

func TestNewReturnsNilWithoutFromEmail(t *testing.T) {
	s, err := New(context.Background(), Config{FromEmail: "  "}, &fakeStore{})
	if err != nil || s != nil || s.Enabled() {
		t.Errorf("s=%v err=%v", s, err)
	}
}

func TestRecordAlternative(t *testing.T) {
	store := &fakeStore{}
	s := newSender(&fakeSES{}, store)
	if err := s.RecordAlternative(context.Background(), "12.345.678/0001-90", "a@b.com", "Brevo", "system"); err != nil {
		t.Fatal(err)
	}
	want := Record{Documento: "12345678000190", Email: "a@b.com", Status: StatusSent, Detail: "Enviado por Brevo (sem confirmação de entrega)", Origem: "system"}
	if len(store.records) != 1 || store.records[0] != want {
		t.Errorf("records = %+v", store.records)
	}
	var disabled *Sender
	if err := disabled.RecordAlternative(context.Background(), "1", "a@b.com", "Brevo", "x"); err != nil {
		t.Errorf("sem SES deveria ser no-op: %v", err)
	}
}

func TestSendReplyToOptional(t *testing.T) {
	ses := &fakeSES{out: &sesv2.SendEmailOutput{MessageId: aws.String("m")}}
	s := NewWithClient(ses, Config{FromEmail: "no-reply@orbeo.site", ReplyTo: "atendimento@orbeo.site"}, &fakeStore{})
	if err := s.Send(context.Background(), Invite{Documento: "1", To: "a@b.com", HTML: "x"}); err != nil {
		t.Fatal(err)
	}
	if got := ses.input.ReplyToAddresses; len(got) != 1 || got[0] != "atendimento@orbeo.site" {
		t.Errorf("Reply-To = %v", got)
	}
	if aws.ToString(ses.input.FromEmailAddress) != "no-reply@orbeo.site" || ses.input.ConfigurationSetName != nil {
		t.Error("sem FromName e sem configuration set o pedido não deveria ter esses campos")
	}

	ses2 := &fakeSES{out: &sesv2.SendEmailOutput{MessageId: aws.String("m")}}
	_ = NewWithClient(ses2, Config{FromEmail: "x@y.com"}, &fakeStore{}).Send(context.Background(), Invite{Documento: "1", To: "a@b.com", HTML: "x"})
	if len(ses2.input.ReplyToAddresses) != 0 {
		t.Error("sem ReplyTo não deve haver Reply-To")
	}
}

package tracer

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// InitTracer inicializa o Provedor de Tracing global e configura o exportador gRPC.
// Deve ser chamado apenas uma vez na inicialização do microserviço (main.go).
//
// serviceName: Nome que aparecerá no dashboard do Jaeger.
// url: Endereço do coletor Jaeger (ex: "IP:PORT").
// Retorna uma função de fechamento que deve ser executada com 'defer'.
func InitTracer(serviceName string, url string) func() {
	ctx := context.Background()

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(url),
	)
	if err != nil {
		log.Fatalf("Falha ao criar exportador OTel: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	otel.SetTracerProvider(tp)
	// Isso permite que o TraceID viaje via HTTP/Mensageria
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Erro ao desligar Tracer Provider: %v", err)
		}
	}
}

// Start inicia um novo Span (transação).
// Se o contexto fornecido já contiver um rastro ativo, o novo Span será criado como filho dele.
//
// origem: Nome da camada ou componente (ex: "http-handler", "service-client").
// name: Descrição da operação (ex: "SaveUser", "GetProductDetails").
// Sempre finalize o Span usando 'defer span.End()'.
func Start(ctx context.Context, origem, name string) (context.Context, trace.Span) {
	// Usar o nome do serviço ajuda a identificar a origem na UI do Jaeger
	return otel.GetTracerProvider().Tracer(origem).Start(ctx, name)
}

// InjectHeaders serializa o contexto de rastreio atual em um mapa de strings.
// Essencial para propagar o rastro para sistemas externos (Filas, Kafka, RabbitMQ).
//
// carrier: O mapa onde os dados do rastro (traceparent) serão injetados.
func InjectHeaders(ctx context.Context, carrier map[string]string) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(carrier))
}

// ExtractHeaders recupera o contexto de rastreio de um mapa de strings vindo de fora.
// Usado em consumidores de fila ou middlewares para manter a continuidade do rastro.
//
// carrier: O mapa contendo os headers recebidos.
func ExtractHeaders(ctx context.Context, carrier map[string]string) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(carrier))
}

# Documentação GO-data-connector-lib

## Descrição Geral
Esta biblioteca escrita em Go foi projetada para simplificar tarefas comuns em projetos, incluindo:

- **Integração com Amazon SQS**: Envio de mensagens para filas SQS com suporte a deduplicidade e configurações personalizadas.
- **Gerenciamento de conexão com bancos de dados**: Criação de conexões com diversos tipos de bancos de dados usando um formato de configuração padronizado.
- **Facilitação de Requisições HTTP**: Execução de requisições HTTP de forma simples e padronizada.

---

## Funcionalidades

### 1. Enviar Mensagens para Amazon SQS

O pacote `queue` facilita o envio de mensagens para filas SQS da AWS. Ele gerencia configurações, autenticação e cria mensagens com duplicidade automática, garantindo consistência no envio de dados.

#### Inicialização do Cliente SQS

Para usar a funcionalidade de envio de mensagens, crie uma instância da estrutura `ToSqs`:

```go
import "github.com/simpplify-org/GO-data-connector-lib/queue"

sqsClient := queue.NewToSqs(
    "AWS_ACCESS_KEY",
    "AWS_SECRET_KEY",
    "AWS_REGION",
    "QUEUE_URL",
)
```

**Parâmetros:**
- `AWS_ACCESS_KEY`: Chave de acesso da AWS.
- `AWS_SECRET_KEY`: Chave secreta da AWS.
- `AWS_REGION`: Região onde a fila está hospedada.
- `QUEUE_URL`: URL da fila SQS.

#### Envio de Mensagens

Use o método `SendMessage` para enviar mensagens para a fila:

```go
message := []byte("Sua mensagem aqui")
messageGroupId := "grupo-de-mensagens"

result, err := sqsClient.SendMessage(message, messageGroupId)
if err != nil {
    log.Fatalf("Erro ao enviar mensagem: %v", err)
}

log.Printf("Mensagem enviada com sucesso: %v", result)
```

**Como funciona:**
- A mensagem precisa ser em bytes.
- É gerado um ID random para evitar duplicidade nas mensagens.
- O método retorna a resposta da AWS com detalhes sobre o envio.

---

### 2. Gerenciar Conexão com Bancos de Dados

O pacote `conn` facilita a configuração e criação de conexões com bancos de dados compatíveis com o pacote `database/sql`.

#### Configuração da Conexão

Crie uma configuração utilizando a estrutura `Config`:

```go
import "github.com/simpplify-org/GO-data-connector-lib/conn"

dbConfig := conn.Config{
    DBDriver:   "postgres", // Ou "mysql", "sqlite", etc.
    DBUser:     "usuario",
    DBPassword: "senha",
    DBHost:     "localhost",
    DBPort:     "5432",
    DBDatabase: "meu_banco",
    DBSSLMode:  "?sslmode=disable", // Opcional, para Postgres
}
```

**Parâmetros:**
- `DBDriver`: Driver do banco (ex.: `postgres`, `mysql`).
- `DBUser`: Usuário para autenticação no banco.
- `DBPassword`: Senha do usuário.
- `DBHost`: Endereço do servidor do banco.
- `DBPort`: Porta para conexão.
- `DBDatabase`: Nome do banco de dados.
- `DBSSLMode`: Parâmetros adicionais de configuração SSL (ex.: `?sslmode=disable`).

#### Criação da Conexão

Use a função `NewConn` para criar uma conexão com o banco de dados:

```go
db, err := conn.NewConn(dbConfig)
if err != nil {
    log.Fatalf("Erro ao conectar ao banco de dados: %v", err)
}
defer db.Close()
```

**Como funciona:**
- A função constrói a string de conexão no formato esperado pelo driver especificado.
- A conexão é estabelecida utilizando o pacote `database/sql`.
- Qualquer erro é retornado imediatamente para tratamento.

---

### 3. Facilitar Requisições HTTP

O pacote `call` fornece uma função para executar requisições HTTP de maneira simplificada, com suporte para cabeçalhos, métodos e corpo de requisição.

#### Realizar uma Requisição HTTP

Use a função `MakeHTTPRequest` para enviar requisições HTTP:

```go
import "github.com/simpplify-org/GO-data-connector-lib/call"

url := "https://api.exemplo.com/endpoint"
method := "POST"
headers := map[string]string{
    "Authorization": "Bearer token_aqui",
    "Custom-Header": "Valor",
}
body := map[string]interface{}{
    "campo1": "valor1",
    "campo2": "valor2",
}

response, err := call.MakeHTTPRequest(url, method, headers, body)
if err != nil {
    log.Fatalf("Erro ao realizar requisição HTTP: %v", err)
}

log.Printf("Status Code: %d", response.StatusCode)
log.Printf("Resposta: %v", response.Body)
```

**Parâmetros:**
- `url`: URL do recurso.
- `method`: Método HTTP (ex.: `GET`, `POST`, `PUT`, `DELETE`).
- `headers`: Map de cabeçalhos para adicionar à requisição.
- `body`: Corpo da requisição, no formato de uma estrutura ou map (serializado automaticamente para JSON).

**Retorno:**
- `*HTTPResponse`: Estrutura contendo:
    - `StatusCode`: Código de status HTTP da resposta.
    - `Body`: Corpo da resposta, deserializado se for JSON.
    - `RawBody`: Corpo da resposta como `[]byte`.
    - `Error`: Erro, se ocorrer.

**Como funciona:**
- Serializa automaticamente o corpo da requisição para JSON.
- Adiciona cabeçalhos necessários, como `Content-Type`, se aplicável.
- Retorna o status HTTP e o corpo da resposta de maneira simples.

---

## Instalação

Adicione a biblioteca ao seu projeto:
```bash
go get github.com/simpplify-org/GO-data-connector-lib
```

---

## Tecnologias utilizadas
Esta biblioteca utiliza as seguintes dependências externas:

- [AWS SDK for Go V2](https://aws.github.io/aws-sdk-go-v2/): Para interações com a AWS.
- [Database/sql](https://pkg.go.dev/database/sql): Para gerenciamento de conexões com bancos de dados.
- [Net/http](https://pkg.go.dev/net/http): Para execução de requisições HTTP.

---

Se precisar de ajustes adicionais, é só avisar!
# GO-data-connector-lib  

Esta biblioteca, desenvolvida em Go, foi criada para simplificar tarefas frequentes em projetos, como a integração com Amazon SQS, permitindo o envio de mensagens para filas com suporte a deduplicidade e 
configurações personalizadas, além do gerenciamento de conexões com diversos bancos de dados através de um formato de configuração padronizado.
---

## Funcionalidades

### 1. Enviar Mensagens para Amazon SQS

O pacote `queue` facilita o envio de mensagens para filas SQS da AWS. Ele gerencia configurações, autenticação e cria mensagens com deduplicidade automática, garantindo consistência no envio de dados.

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
messageGroupId := "grupo-de-mensagens"]

result, err := sqsClient.SendMessage(message, messageGroupId)
if err != nil {
    log.Fatalf("Erro ao enviar mensagem: %v", err)
}

log.Printf("Mensagem enviada com sucesso: %v", result)
```

**Como funciona:**
- A mensagem precisar ser em bytes.
- É gerado um ID random para evitar deduplicidade nas mensagens.
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

## Tecnologias utilizadas

- Golang: Linguagem principal para implementação.
- [AWS SDK for Go V2](https://aws.github.io/aws-sdk-go-v2/): Biblioteca para interação com serviços da Amazon Web Services (AWS), incluindo Amazon SQS.
- [Database/sql](https://pkg.go.dev/database/sql):: Pacote nativo do Go para gerenciamento de conexões com bancos de dados.

---

## Instruções para Executar

### Execução Local

1. **Adicione a biblioteca ao seu projeto:**
    ```bash
    go get github.com/simpplify-org/GO-data-connector-lib
    ```




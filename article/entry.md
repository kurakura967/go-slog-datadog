# [Golang] slogを使ってDatadogにログを送ってみる
## 目次
- 本記事でやること
- 対象読者
- 使用言語とライブラリー
- 背景
- 環境構築
  - Datadogのアカウント作成
  - Datadog-AgentをDockerで起動
  - DatadogのWEB UIからAgentが起動していることを確認
  - アプリケーションのDockerコンテナな起動
- slogでログを送る
  - Datadogのログを確認する
  - log.Printfでログを送った時の違い
- トレースをDataDogに送る
  - Datadogのトレースを確認する
- ログとトレースの接続
  - withでtrace_idをinjectするように実装する
  - handlerでtrace_idをinjectするように実装する
- まとめ

## 本記事でやること
- Datadog-AgentのDockerコンテナを起動する
- ログ収集対象のアプリケーションのDockerコンテナを起動する
- slogを用いてDataDogにログを送る
- トレースをDataDogに送る
- ログにトレースIDを付与する

今回実装したコードはこちらのレポジトリで公開しております。

## 対象読者
- (難しいことは一旦置いておいて)Datadogを触ってみたい方
- slogを使ってみたい方

## 使用言語
- Go言語 1.21.0

## 背景
log/slog(以下slog)はGo1.21から標準ライブラリに含まれるようになった構造化ロギングパッケージです。
構造化ログギングには3rdパーティライブラリを利用するケースが多く、slogへの置き換えも積極的には行われていないように感じます。
しかし、今後アプリケーションを開発する上で標準ライブラリであるslogを選択していくことは、アプリケーションのメンテナンス性を高める上で非常に重要なことだと考えています。
また、今後の業務に活かすことを考え、Datadogを用いながらslogを利用する方法をまとめました。


## 環境構築

### アカウントの作成

まずはDatadogのアカウントを作成します。「無料トライアル」であれば14日間無料で利用できます。

https://www.datadoghq.com/ja/pricing/

セットアップを進めていくと以下の画面にてAPIキーが発行されます。このAPIキーを後ほど利用します。

![datadog-setup](./images/datadog-setup.png)


### Datadog-AgentをDockerで起動
DockerでDatadog-Agentを起動します。今回は監視対象のアプリケーションもDockerコンテナで起動するため、Docker Composeを利用します。
まずは以下docker-compose.ymlを作成します。

```yml:docker-compose.yml
version: "3"
services:
  datadog-agent:
    container_name: datadog-agent
    image: gcr.io/datadoghq/agent:latest
    pid: host
    environment:
      - DD_API_KEY=<YOUR_API_KEY>
      - DD_SITE=ap1.datadoghq.com
      - DD_APM_NON_LOCAL_TRAFFIC=true
      - DD_APM_ENABLED=true
      - DD_LOGS_ENABLED=true
      - DD_LOGS_CONFIG_CONTAINER_COLLECT_ALL=true
      - DD_AC_EXCLUDE=name:datadog
      - DD_LOGS_CONFIG_USE_HTTP=true
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /proc/:/host/proc/:ro
      - /sys/fs/cgroup:/host/sys/fs/cgroup:ro
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
    ports:
        - "8126:8126/tcp"
        - "8125:8125/udp"
```

以下を実行し、Datadog-Agentのコンテナを起動します。
```bash
$ docker compose build --no-cache
$ docker compose up -d
$ docker compose ps
datadog-agent       "/bin/entrypoint.sh"   datadog-agent       running (healthy)   0.0.0.0:8125->8125/udp, :::8125->8125/udp, 0.0.0.0:8126->8126/tcp, :::8126->8126/tcp
```
無事Dockerコンテナが起動したことが確認できたら、DatadogのWEB UIからAgentが起動していることを確認します。
WEBUIにログインし、左メニューの「Infrastructure」を選択し、「docker-desktop」がActiveになっていることが確認できればOKです。
![datadog-webui-infra](./images/datadog-infra.png)


### アプリケーションのDockerコンテナな起動

Datadog-AgentのDockerコンテナが起動できたら、次は監視対象のアプリケーションのDockerコンテナを起動します。
今回は簡略化のため、以下のような"Hello World"を返すだけのHTTPサーバーを作成します。
また、[こちら](https://docs.datadoghq.com/ja/tracing/trace_collection/library_config/go/)の公式ドキュメントを参考にトレーシングライブラリーを利用し、APMデータを収集できるようにします。

```go:main.go
package main

import (
	"log"
	"net/http"

	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func main() {

	tracer.Start(
		tracer.WithService("sample-service"),
		tracer.WithEnv("dev"),
		tracer.WithDebugMode(true),
	)
	defer tracer.Stop()

	// Create a traced mux router
	mux := httptrace.NewServeMux()
	
	// Continue using the router as you normally would.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World!!!\n"))
	})
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
```

次に、Dockerfileを作成します。また、今回は開発効率を考えairを利用してホットリロードできるようにしています。

```Dockerfile:Dockerfile
FROM golang:1.21.0

WORKDIR /home/go-slog-datadog

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main .

ENV DD_TRACE_SAMPLE_RATE 1

RUN go install github.com/cosmtrek/air@latest

CMD ["air", "-c", ".air.toml"]
```

これで、アプリケーションとDatadog-AgentのDockerコンテナを起動する準備が整いました。
docker-compose.yamlにアプリケーションのDockerコンテナを追加します。

```yml:docker-compose.yml
version: "3"
services:
  app:
    container_name: app
    build:
        context: .
        dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - DD_SERVICE=sample-service
      - DD_ENV=dev
      - DD_VERSION=0.0.1
      - DD_AGENT_HOST=datadog-agent
    volumes:
      - .:/home/go-slog-datadog
    depends_on:
      - datadog-agent

  datadog-agent:
    ...(省略)
```


## slogでログを送る
log/slog(以下slog)はGo1.21から標準ライブラリに含まれるようになった構造化ロギングパッケージです。
今回はslogの内部構造に関する詳細は他の文献に任せ、各種メソッドを利用しDatadog上に出力されるログを確認しながらslogの外観を確認していきます。

```go:main.go
func main() {
    ....(省略)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		span, _ := tracer.StartSpanFromContext(r.Context(), "GetHelloWorldHandler")
		defer span.Finish()
        
        // Hello Worldを出力
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true}))
		logger.Info("Hello World!!!")

		w.Write([]byte("Hello World!!!\n"))
	})
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
```
標準出力には以下のように構造化されたログが出力されます。
```json
{"time":"2023-12-15T01:39:33.1268174Z","level":"INFO","source":{"function":"main.main.func1","file":"/home/go-slog-datadog/main.go","line":33},"msg":"Hello World!!!"}
```

それでは、Datadog上にログが出力されているか確認してみましょう。
DatadogのWEB UIから「Logs」を選択し、ログが出力されていることを確認します。
以下の通り、「Event Attributes」に構造化されたログが出力されていることが確認できます。
![datadog-webui-logs](./images/datadog-logs1.png)

また、今回はDatadogのAPM機能を利用しトレースを収集できるようにしています。こちらもWEB UIから確認できます。

![datadog-webui-apm](./images/datadog-apm.png)


## ログとトレースの接続
ログとトレースを接続することで、エラーの発生箇所やパフォーマンスの低下の原因の特定が容易になります。
ここでは、slogのログにトレースIDとスパンIDを追加することで、ログとトレースを接続します。

slogは以下のように`Handler`インターフェースを持ちます。

https://pkg.go.dev/log/slog#Handler

```go
type Handler interface {
    Enabled(context.Context, Level) bool
    Handle(context.Context, Record) error
    WithAttrs(attrs []Attr) Handler
    WithGroup(name string) Handler
}
```
Handleメソッドは、`context.Context`型と`slog.Record`型を引数に取り、最終的なログの出力を担うメソッドです。
Record構造体は、時刻やメッセージ、ログレベルなどの情報を持ちます。
このRecord構造体はAddメソッドを持ち、任意の属性値を追加することができます。

https://pkg.go.dev/log/slog#Record

また、`dd-trace-go`パッケージの`SpanFromContext`メソッドを用いて`context.Context`からトレースIDとスパンIDを取得します。
```go
import (
    "context"
    "log/slog"

    "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type DatadogHandler struct {
	slog.Handler
}

func NewDatadogHandler(s slog.Handler) slog.Handler {
	return &DatadogHandler{
		s,
	}
}

func (h *DatadogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

func (h *DatadogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &DatadogHandler{h.WithAttrs(attrs)}
}

func (h *DatadogHandler) WithGroup(name string) slog.Handler {
	return &DatadogHandler{h.WithGroup(name)}
}

func (h *DatadogHandler) Handle(ctx context.Context, r slog.Record) error {
	span, ok := tracer.SpanFromContext(ctx)
	defer span.Finish()
	if ok {
		// spanがある場合は、spanの情報をlogに付与する
		traceId := span.Context().TraceID()
		spanId := span.Context().SpanID()
		group := slog.Group("dd",
			slog.Uint64("trace_id", traceId),
			slog.Uint64("span_id", spanId),
		)

		r.Add(group)
	}
	return h.Handler.Handle(ctx, r)
}
```

トレースIDとスパンIDをログに付与する際は、以下公式ドキュメントにあるように、`dd`というキーを持つオブジェクトを作成し、その中に`trace_id`と`span_id`を追加します。

> Datadog ログインテグレーションを使ってログをパースしていない場合は、カスタムログパースルールによって dd.trace_id、dd.span_id、dd.service、dd.env、dd.version が文字列としてパースされていることを確実にする必要があります。詳しくは、関連するログがトレース ID パネルに表示されないを参照してください。

https://docs.datadoghq.com/ja/tracing/other_telemetry/connect_logs_and_traces/go/

上記の実装をアプリケーションに組み込み、Datadogにログを送信します。
`MyFunc`関数の中でログを出力するようにしています。MyFuncの呼び出し側で渡されたcontext.Contextを元にトレースIDを取得するため、slog.InfoContextメソッドを用います。

https://pkg.go.dev/log/slog#hdr-Contexts

```go:main.go
func main() {

	tracer.Start(
		tracer.WithService("sample-service"),
		tracer.WithEnv("dev"),
		// tracer.WithAgentAddr("datadog-agent:8126"),
		tracer.WithDebugMode(true),
	)
	defer tracer.Stop()

	// Create a traced mux router
	mux := httptrace.NewServeMux()
	// Continue using the router as you normally would.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		span, ctx := tracer.StartSpanFromContext(r.Context(), "GetHelloWorldHandler")
		defer span.Finish()
		
		MyFunc(ctx)

		w.Write([]byte("Hello World!!!\n"))
	})
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func MyFunc(ctx context.Context) {
	span, _ := tracer.StartSpanFromContext(ctx, "MyFunc")
	defer span.Finish()
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true})
	datadogHandler := NewDatadogHandler(jsonHandler)
	logger := slog.New(datadogHandler)
	logger.InfoContext(ctx, "logger from MyFunc")
}
```

標準出力は以下のようになります。
```json
{"time":"2023-12-15T05:19:09.6896378Z","level":"INFO","source":{"function":"main.MyFunc","file":"/home/go-slog-datadog/main.go","line":47},"msg":"logger from MyFunc","dd":{"trace_id":1872485691139412764,"span_id":7737792784606034605}}
```

以下のようにAPMの画面からトレースIDに紐づくログを確認することができます。

![datadog-webui-apm2](./images/datadog-apm2.png)

また、ログの画面からトレースIDによってログを検索することができます。

![datadog-webui-log2](./images/datadog-logs2.png)


## まとめ
今回はDatadogの環境構築から、slogパッケージを用いたログ送信・トレースとログの接続までを行いました。
ログにトレースIDとスパンIDを付与するため、slogのHandlerインターフェイスやHandleメソッドについて理解することができました。
また、DatadogのWEB UIでログやトレースを確認する中でDatadogの機能についても理解することができました。
本番環境にslogを導入するためのベースとなる記事となれば幸いです。

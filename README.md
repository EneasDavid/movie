# df-orfeu

Player de vídeo estilo streaming (play/pause, avançar/voltar 15s, barra de
progresso com buffer, catálogo em linhas) para um catálogo hospedado numa
pasta do Google Drive. Um único usuário, sem login/senha.

## Stack

| Camada | Tecnologia | Por quê |
|---|---|---|
| Backend | Go (stdlib `net/http`) | Baixo consumo de memória, concorrência nativa (goroutines) ideal pra proxy de streaming e cache |
| Frontend | React + React Router | Base para reaproveitar lógica num futuro app mobile (React Native) |
| Estilos | SCSS (Sass) | Nesting, mixins e variáveis em build-time; compila pra CSS puro — zero custo em runtime |
| Build frontend | Vite | Build rápido, bundling com hash de conteúdo |
| Cache / rate limit | Upstash Redis (REST API) | Funciona sobre HTTPS puro, sem conexão TCP persistente — necessário porque a Vercel roda o Go de forma efêmera |
| Catálogo | Google Drive API v3 (API key) | Sem OAuth — a pasta precisa estar pública ("Qualquer pessoa com o link") |
| Deploy | Vercel (Go Framework Preset) | Detecta `cmd/server/main.go`, builda e roda o servidor Go direto |
| CI | GitHub Actions | Build + vet + lint + gofmt antes de qualquer coisa chegar à `main` |

## Como rodar localmente

Pré-requisitos: Go 1.22+, Node 20+.

```bash
cp .env.example .env   # preencha GOOGLE_DRIVE_API_KEY (veja abaixo)
make build             # builda o React (Vite) e depois o binário Go
make run               # sobe em http://localhost:8080
```

Ou, para desenvolvimento com hot-reload dos dois lados (dois terminais):

```bash
make dev-server   # Go em :8080
make dev-web      # Vite em :5173, com proxy de /api/* pro :8080
```

### Obtendo a API key do Google Drive

1. [console.cloud.google.com/apis/credentials](https://console.cloud.google.com/apis/credentials) → **Create credentials → API key**.
2. Ative a **Google Drive API** no mesmo projeto (APIs & Services → Library).
3. Restrinja a key à Drive API (recomendado, não obrigatório).
4. **A pasta do Drive precisa estar compartilhada como "Qualquer pessoa com o link" (Leitor).** Sem OAuth/service account, a API key só enxerga o que é público — é a única causa de "catálogo vazio" quando a key e o `FOLDER_ID` estão corretos.

### Redis (Upstash) — recomendado, mas opcional

O app funciona sem Redis (degrada graciosamente: sem cache, sem rate limit
persistente entre instâncias), mas com ele:

1. Vercel → **Storage → Create Database → Upstash** (injeta as env vars
   automaticamente), ou crie direto em [upstash.com](https://upstash.com)
   (grátis) e copie a REST URL/token.
2. Configure a política de **Eviction** do banco como `allkeys-lru`, para
   que o cache de vídeo se auto-limite sob pressão de memória em vez de
   dar erro.
3. Local: `vercel env pull .env.local` ou preencha `UPSTASH_REDIS_REST_URL`
   / `UPSTASH_REDIS_REST_TOKEN` manualmente no `.env`.

## Deploy (Vercel)

O projeto já está conectado a um repositório GitHub com deploy automático
(a Vercel builda a cada push na `main`). O `vercel.json` define:

```json
"buildCommand": "npm --prefix web ci && npm --prefix web run build && go build -o server ./cmd/server"
```

Ou seja: builda o React primeiro (gera os arquivos em
`internal/web/static`), depois o Go, que **embute** esses arquivos no
binário via `go:embed` (`internal/web/embed.go`). Um único binário serve
tanto a API quanto o frontend — nada de build separado por função
serverless.

Configure em **Project Settings → Environment Variables**:
`GOOGLE_DRIVE_API_KEY`, `GOOGLE_DRIVE_FOLDER_ID`, e (se optar por Redis)
`UPSTASH_REDIS_REST_URL` / `UPSTASH_REDIS_REST_TOKEN`.

Para enviar confirmações e redefinições de senha a qualquer usuário, o
domínio configurado no Resend precisa aparecer como **Verified** (não
**Pending**) e o deploy deve usar, por exemplo,
`EMAIL_FROM="df-orfeu <df-orfeu@naoresponder.ic.ufal.br>"`. O remetente de
teste `onboarding@resend.dev` só entrega para o endereço do proprietário da
conta Resend.

## Arquitetura

```
web/                        React + SCSS (fonte) — Vite builda pra internal/web/static
  src/
    pages/                  Catalog.jsx, Watch.jsx (rotas do React Router)
    components/catalog/     Hero, Row, Card
    components/player/      VideoPlayer, Controls, CenterFeedback
    hooks/                  useIdleControls (auto-hide dos controles)
    lib/                    api.js (chamadas HTTP), progress.js (localStorage)
    styles/                 tokens/mixins/partials SCSS

cmd/server/main.go          Entrypoint único: rotas /api/* + fallback SPA
internal/
  appctx/                   Monta config+cliente Drive+Redis uma vez por instância (sync.Once)
  config/                   Leitura de env vars (+ loader de .env pro dev local)
  drive/                    Cliente da Drive API (catálogo, metadata, streaming, retry)
  store/                    Cliente Redis (REST) + rate limiter + cache de catálogo/thumb/chunk
  middleware/                Guard: headers de segurança + método + rate limit, uma vez só
  httpx/                    Headers de segurança e resposta de erro em JSON padronizada
  handlers/                 Lógica de cada rota (Catalog, Stream, Thumbnail, Health)
  web/                      go:embed do build do React
```

## Técnicas usadas (e por quê)

**Cache em camadas (Redis via Upstash REST)**
- *Catálogo*: 1 blob JSON, TTL de 5 min — evita percorrer a árvore de
  pastas do Drive a cada visita.
- *Thumbnails*: bytes da imagem cacheados (TTL 30 min) — a Drive expõe
  links de thumbnail efêmeros; buscamos uma vez e servimos do nosso
  domínio dali em diante.
- *Chunks de vídeo*: o proxy de streaming (`/api/stream`) fatia o vídeo em
  blocos de 1MB e cacheia cada bloco individualmente (TTL 20 min).
  Reassistir do início, ou dois acessos à mesma cena, batem no cache em
  vez de re-baixar do Drive. Requisições de range grandes/não-alinhadas
  (a maior parte da reprodução contínua) passam direto (pass-through),
  já que cachear um blob assistido uma única vez não ajudaria em nada.

**Rate limiting**
- Janela fixa por IP via `INCR`+`EXPIRE` no Redis — funciona mesmo com o
  servidor rodando em múltiplas instâncias efêmeras (a Vercel não garante
  a mesma instância entre requisições).
- Um semáforo local (em memória, por instância) limita chamadas
  concorrentes ao Drive, evitando estourar a cota da API em rajadas
  (várias abas, seeks rápidos).
- Tudo isso falha *aberto*, não fechado: se o Redis estiver indisponível
  ou não configurado, a aplicação segue funcionando sem cache/rate limit
  em vez de derrubar o serviço inteiro.

**Idempotência**
- `GET /api/catalog`, `/api/stream`, `/api/thumbnail`, `/api/health` são
  todos idempotentes por natureza (leitura pura, sem efeito colateral).
  Cachear e re-cachear o mesmo catálogo em paralelo (corrida entre
  requisições concorrentes num cache frio) só sobrescreve a mesma chave
  Redis com dado equivalente — seguro por design, sem lock necessário.
- `writeProgress()` no frontend (localStorage) é idempotente pelo mesmo
  motivo: grava `{time, duration}` do zero a cada chamada, nunca
  incrementa/acumula.

**Concorrência (Go)**
- `BuildCatalog` busca todas as subpastas (cada uma vira uma "linha" do
  catálogo) **em paralelo** via goroutines + `WaitGroup`, limitado pelo
  semáforo — o tempo total fica perto do da subpasta mais lenta, não da
  soma de todas.

**Segurança**
- A API key do Google **nunca** chega ao navegador — todo acesso ao Drive
  passa pelo backend, inclusive o streaming (proxy, não redirect).
- Validação de `fileId` via regex antes de tocar em qualquer chave de
  cache, URL do Drive, ou mensagem de erro (evita injeção).
- Headers de segurança em toda resposta: CSP restrita a `'self'`,
  `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
  `Referrer-Policy: same-origin`.
- Mensagens de erro para o cliente são sempre genéricas; o erro real vai
  pro log do servidor (visível no dashboard da Vercel), nunca pra
  resposta HTTP.

**Streaming eficiente**
- Suporte completo a `Range`/`Accept-Ranges` — o `<video>` do navegador
  faz seek nativamente, sem JS customizado pra isso.
- `HEAD` tratado à parte (sem corpo), como qualquer CDN de vídeo real.

## Frontend: interações

- **Play/pause**: clique no vídeo, botão dedicado, ou `Espaço`/`K`.
- **Voltar/avançar 15s**: botões dedicados ou `←`/`→` — cada clique é
  independente, então cliques repetidos em sequência acumulam
  normalmente (não há "lock" entre eles).
- **Barra de progresso**: mostra buffer (cinza) e progresso (vermelho)
  separadamente; clicável e arrastável para seek.
- **Continuar assistindo**: progresso salvo no `localStorage` a cada ~5s
  e ao trocar de aba/fechar; retomado automaticamente ao reabrir o mesmo
  vídeo (linha própria no catálogo).
- **Atalhos**: `F` tela cheia, `M` mudo, `↑`/`↓` volume.
- **Animações**: seguem as regras de
  [emilkowalski/skills](https://github.com/emilkowalski/skills) — curvas
  `cubic-bezier` fortes (não os `ease` genéricos do CSS), apenas
  `transform`/`opacity`, hover restrito a ponteiros precisos
  (`@media (hover: hover) and (pointer: fine)`, touch não dispara hover
  falso), e `prefers-reduced-motion` respeitado em toda animação.

## Problema conhecido: vídeo não reproduz (áudio incompatível)

Se o player mostrar "arquivo de origem usa um formato de áudio
incompatível com o navegador", o `/api/stream` está funcionando
corretamente (200, bytes corretos) — o problema é que o arquivo de vídeo
em si tem a trilha de áudio em **E-AC-3** (ou outro codec sem suporte em
navegadores), e isso não é algo que dá pra corrigir no app: nenhum
navegador tem decoder de E-AC-3, então o `<video>` recebe o arquivo certo
e mesmo assim recusa a decodificar. Não é bug de rede, de auth nem de
compartilhamento do Drive.

Transcodificação em tempo real no servidor foi considerada e descartada:
o deploy do Vercel ("Go Framework Preset") não tem `ffmpeg` disponível no
runtime, e mesmo que tivesse, recodificar um filme inteiro em tempo real
a cada request estouraria os limites de execução/memória do Vercel de
longe.

A correção real é uma vez só, na origem — duas formas de fazer isso,
ambas rodando localmente (nenhuma roda dentro do app publicado, pelo
motivo acima):

### Opção A — manual: baixa, roda o script, sobe de volta

[`scripts/fix-audio-codec.sh`](scripts/fix-audio-codec.sh) remuxa
arquivos locais (requer `ffmpeg`/`ffprobe` — `brew install ffmpeg`).
Copia o vídeo sem recodificar (rápido, sem perda de qualidade) e
converte só o áudio pra AAC:

```bash
./scripts/fix-audio-codec.sh /caminho/para/os/videos /caminho/de/saida
```

Depois é você quem sobe os arquivos de saída pro Drive no lugar dos
originais.

### Opção B — automático: direto contra o Drive

[`cmd/fixaudio`](cmd/fixaudio/main.go) faz tudo isso sozinho: lista o
catálogo inteiro (pasta raiz + subpastas, mesma estrutura que vira
categorias no app), baixa cada vídeo, confere o codec de áudio, remuxa o
que estiver incompatível e **sobe o resultado de volta no mesmo file ID**
— por isso o catálogo, o cache de thumbnail e o progresso salvo de
"continuar assistindo" continuam válidos sem precisar mexer em mais
nada.

O workflow `.github/workflows/fix-incompatible-media.yml` executa essa
varredura diariamente e também pode ser iniciado em **Actions → Fix
incompatible media → Run workflow**. Configure estes secrets no repositório:

- `GOOGLE_DRIVE_FOLDER_ID`: ID da pasta raiz do catálogo.
- `GOOGLE_SERVICE_ACCOUNT_JSON_B64`: conteúdo do JSON da conta de serviço
  codificado com `base64 < chave.json | tr -d '\n'`.

A pasta raiz precisa estar compartilhada como **Editor** com o
`client_email` do JSON. Sem isso, o download funciona, mas a atualização do
arquivo termina com `403 insufficientFilePermissions`.

Isso precisa de permissão de **escrita** no Drive, que a API key atual
não tem (ela só lê). Configuração única, no
[Google Cloud Console](https://console.cloud.google.com/):

1. No mesmo projeto onde a API key do Drive foi criada, vá em **IAM e
   administrador → Contas de serviço → Criar conta de serviço**. Não
   precisa nenhuma permissão especial de projeto/IAM — o acesso vem de
   compartilhar a pasta do Drive com ela, no próximo passo.
2. Abra a conta de serviço criada → aba **Chaves → Adicionar chave →
   Criar nova chave → JSON**. Isso baixa um arquivo `.json` — guarde-o
   fora do repositório (nunca faça commit dele).
3. Copie o email da conta de serviço (termina em
   `...iam.gserviceaccount.com`). No Google Drive, abra a pasta raiz do
   catálogo → **Compartilhar** → cole esse email → permissão **Editor**.
   Sem esse passo a conta de serviço não enxerga a pasta, mesmo com a
   chave em mãos — é o mesmo modelo de compartilhar com qualquer pessoa.

Com isso feito:

```bash
go run ./cmd/fixaudio -credentials=/caminho/para/service-account.json -folder=$GOOGLE_DRIVE_FOLDER_ID
```

Use `-dry-run` primeiro pra ver o que seria corrigido sem baixar/subir
nada.

Depois, suba os arquivos de `/caminho/de/saida` pro Google Drive no lugar
dos originais (e apague os antigos, pra não duplicar no catálogo).

## Próximos passos sugeridos

- **App mobile**: o backend já é uma API JSON pura
  (`/api/catalog`, `/api/stream`, `/api/thumbnail`) — um app React Native
  consumiria exatamente os mesmos endpoints, sem mudar nada aqui.
- Testes automatizados (`go test`) — a estrutura em `internal/handlers`
  já isola a lógica de HTTP puro, facilitando testes de unidade.

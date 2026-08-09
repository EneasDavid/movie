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

## Próximos passos sugeridos

- **App mobile**: o backend já é uma API JSON pura
  (`/api/catalog`, `/api/stream`, `/api/thumbnail`) — um app React Native
  consumiria exatamente os mesmos endpoints, sem mudar nada aqui.
- Testes automatizados (`go test`) — a estrutura em `internal/handlers`
  já isola a lógica de HTTP puro, facilitando testes de unidade.

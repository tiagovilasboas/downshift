<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Spike: o KiroCrew depende do kiro-cli?

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Data:** 2026-10-26 · **Pergunta:** um usuário que tem só o KiroCrew instalado, sem o
`kiro-cli` no PATH, consegue usar o Downshift com o KiroCrew? E o `kiro-cli` funciona
sem o KiroCrew?

## Resposta curta

- **O KiroCrew depende de um kiro-cli funcionando.** O app traz uma cópia própria, fixada
  na versão 2.27.1, em `Contents/Resources/backend-dist/kiro-cli/kiro-cli-chat`. Não há
  um kiro-cli "solto" obrigatório para o app existir, mas o app precisa que essa cópia (ou
  a instalada no PATH) responda a uma sonda. Sem resposta, ele recusa `/api/models` e o uso.
- **O Downshift só sabe listar modelos pelo kiro-cli do PATH.** Quem só tem o KiroCrew
  não tem essa lista, e o Downshift segura o roteamento do KiroCrew. Não reescreve nada
  errado, mas não roteia.
- **O kiro-cli é um produto próprio.** Instalado em `~/.local/bin` ele funciona sem o
  KiroCrew. Não foi testado aqui, porque o kiro-cli foi removido nesta máquina.

## Evidências

| Achado | Fonte | Consequência |
|---|---|---|
| Cópia do kiro-cli embutida no app, versão 2.27.1 | `KiroCrew.app/.../backend-dist/kiro-cli/` (`BUNDLED-VERSION`, `kiro-cli-chat`) | O app não depende de instalação separada para ter o binário |
| O gateway aponta para a cópia embutida | `app.asar`: "Point the gateway at the kiro-cli copy staged into the app's own resources" | Ela é o caminho que o app usa |
| Sonda de pré-requisito por caminho | `gateway-launch.log`: `kiro_prerequisite` avisa que `~/.local/bin/kiro-cli` e a cópia embutida "could not be verified: the probe timed out after 10s" | O app distingue ausente de "não verificado" |
| `/api/models` e `/api/sessions/usage` recusados com 503 | `gateway-launch.log`: `kiro_prerequisite_required`, em 09:27, 16:45 e 19:31 | Sem a sonda verde, o KiroCrew não mostra modelos nem uso |
| Downshift lista só pelo PATH | `internal/discovery/sources.go`, `KiroCLI{Bin}` com default `kiro-cli` | Usuário só com o KiroCrew fica sem lista |

## O que isso significa

1. **O risco é real, mas tem duas partes.** No Downshift, a lista do KiroCrew some sem o
   kiro-cli no PATH. No KiroCrew, a sonda pode falhar mesmo com o binário presente (timeout
   de 10 s). Isso já aparecia nos logs antes da desinstalação.
2. **Corrigir só o Downshift não basta.** Se a sonda do KiroCrew falha, a lista vinda da
   cópia embutida também não é confiável. Antes de depender dela, é preciso saber por que a
   sonda estoura o tempo.
3. **O mais provável para a sonda estourar:** a cópia embutida precisa de login ativo. O
   primeiro log de timeout é de um caminho do PATH que hoje não existe mais.

## O que não foi provado

- Se a cópia embutida lista modelos sem login. Testar isso pode abrir uma aba de login
  (foi a causa do loop de abas), então precisa de autorização antes.
- Se um subagente do KiroCrew roda com o kiro-cli removido. Não foi testado.
- Se o `kiro-cli` standalone funciona sem o KiroCrew. Não foi testado nesta máquina.

## Proposta

1. **No Downshift (código):** a fonte `KiroCLI` procura o kiro-cli no PATH e, se não achar,
   usa a cópia embutida no `KiroCrew.app`. A lista continua vindo da CLI, nunca fixa. Se
   nenhuma responder, o hook segura, como hoje.
2. **No doctor:** quando o KiroCrew não tem lista, o aviso diz qual é o motivo (CLI ausente,
   sonda sem resposta ou login pendente) e a dica certa, em vez de sugerir `models discover`.
3. **Teste ao vivo, com autorização:** rodar a cópia embutida com `--list-models` uma vez,
   com o usuário presente, para confirmar se lista sem abrir aba de login.
4. **Decisão do dono:** reinstalar o kiro-cli standalone ou manter só a cópia do KiroCrew.

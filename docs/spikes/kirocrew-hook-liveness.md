<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Spike: o KiroCrew chama o hook do Downshift num spawn real?

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Data:** 2026-10-26 · **Status:** aberto — falta prova operacional · **Dono:** Downshift (hook e doctor).

## Pergunta

Quando o KiroCrew dispara um subagente por **spawn_run**, o hook **preToolUse** do Downshift é executado? A execução gera uma decisão de roteamento que corresponde ao modelo resolvido pelo KiroCrew?

## Evidência disponível

| Fato | Evidência | Limite |
|---|---|---|
| O setup do Downshift declara **preToolUse** para todas as ferramentas e **postToolUse** para **spawn_run** no KiroCrew | **cmd/downshift/setup_targets.go** e testes do setup | Configuração versionada prova a intenção, não a chamada em runtime |
| O wrapper procura o executável configurado e faz fallback para o **downshift** disponível no PATH | **cmd/downshift/setup_wrapper.go** e testes | O ambiente real do gateway ainda pode divergir |
| O doctor valida a configuração do hook e procura eventos de roteamento | **cmd/downshift/doctor_hooks.go** | Um hook executado sem decisão não gera evidência própria |
| O runner registra eventos depois de obter uma decisão de roteamento | **cmd/downshift/hookrun.go** | Ausência do evento não distingue hook ausente, payload ignorado e falha de escrita |
| Um spawn anterior iniciou um subagente, mas não deixou evento do Downshift | Registro operacional da investigação | Sem batimento, não prova se o hook foi chamado |
| Existe **downshift probe**, que registra campos selecionados e hashes de IDs | **cmd/downshift/probe.go** e testes | É opt-in e não comprova a execução normal do hook |

## Hipóteses

| # | Hipótese | Evidência que a diferencia |
|---|---|---|
| H1 | O KiroCrew não chama **preToolUse** para um **spawn_run** iniciado no chat | Não aparece batimento específico de **spawn_run** após uma chamada real |
| H2 | O hook roda, mas o payload não produz uma decisão ou a gravação falha | O batimento de **spawn_run** aparece sem evento de roteamento |
| H3 | O gateway executa um comando antigo ou indisponível | Doctor e log do gateway mostram o comando que foi encontrado ou a falha |
| H4 | O ambiente impede a persistência do batimento | O hook é identificado, mas o registro falha; uma comparação de sandbox exigiria decisão separada |

## Contrato proposto

- Registrar uma evidência pequena e limitada por chamada do hook, antes de depender da decisão de roteamento.
- Guardar somente horário UTC, harness registrado e nome de ferramenta validado. Não gravar prompt, tarefa, caminho, ID de sessão ou payload.
- Limitar crescimento e permissões do armazenamento. Falha de escrita não muda a resposta do hook nem bloqueia o harness.
- O doctor deve separar “hook configurado”, “chamada observada” e “decisão de roteamento observada”. Deve identificar especificamente a idade do último **spawn_run**, para que chamadas de outras ferramentas não escondam sua ausência.

## Experimento operacional

1. Rodar o doctor antes do teste e confirmar a configuração e o binário do hook.
2. Manter a descoberta automática desligada (**DOWNSHIFT_DISCOVERY=off**) para que o teste não chame a descoberta de modelos.
3. Pedir explicitamente uma única chamada **spawn_run** no KiroCrew para comparar, em leitura, o setup, o runner e o doctor; não usar o pipeline nativo nem alterar arquivos.
4. Conferir, nesta ordem: batimento **spawn_run**; evento de roteamento com **requested_model** e **final_model**; modelo **resolved_model** no registro nativo do subagente.
5. Se o hook for chamado e negar o spawn, o batimento prova liveness, mas ainda não prova execução nem correspondência de modelo. Não repetir indefinidamente.
6. Não mudar configuração de sandbox ou outra proteção durante este experimento. Se a hipótese H4 exigir comparação entre modos, abrir uma decisão específica antes de alterar a configuração.

## Critérios de conclusão

- Testes determinísticos cobrem chamada válida, payload inválido, ausência de modelo e falha de escrita; o hook continua fail-open e a saída do harness permanece compatível.
- O registro é limitado e não contém prompt, tarefa, caminho, IDs ou conteúdo do payload.
- **downshift doctor** informa configuração, chamada recente e **spawn_run** recente sem confundir esses estados com uma decisão de roteamento.
- Um spawn real produz batimento e decisão; o **resolved_model** reportado pelo KiroCrew corresponde ao **final_model** registrado pelo Downshift.
- Se o spawn real não puder ser executado, registrar o bloqueio observado e manter o status aberto. Teste unitário não substitui a prova em runtime.

## Guia e sensor (Harness Fowler)

- **Guia inferencial, eixo behaviour:** **docs/guides/kirocrew-routing-guide.md** explica a chamada recomendada e como interpretar os sinais do doctor.
- **Sensor computacional, eixo behaviour:** batimento limitado, estado específico de **spawn_run**, testes automatizados e CI.
- **Sensor inferencial, eixo maintainability/architecture fitness:** revisão do limite de privacidade, fail-open e separação de responsabilidades.

## Fora de escopo

- Alterar modelo ou comando no KiroCrew; o hook permite ou bloqueia, não reescreve a chamada.
- Investigar a lista de modelos 5-5, reporte de tokens, loop de login ou publicação/release.
- Alterar sandbox ou outras configurações de segurança do KiroCrew.

<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Tarefas: liveness do hook KiroCrew

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Esta lista transforma o [spike](../../../spikes/kirocrew-hook-liveness.md) em incrementos verificáveis. Os pontos usam Fibonacci e cada tarefa fica no teto de 5.

## K0 — Artefatos e contrato de aceite — 2 SP

**Dono:** root. **Estado:** escrito localmente; aguarda PR.

- Atualizar spike e roadmap e criar esta lista antes de implementar.
- Remover caminhos, IDs de sessão e detalhes de execução locais do stub público.
- Deixar explícito que ausência de evento não prova ausência de chamada.

**Aceite:** os três artefatos apontam para o mesmo contrato, hipóteses, dependências e critérios de conclusão.

## K1 — Separar responsabilidades dos arquivos grandes — 4 SP

**Dono:** Backend Platform. **Dependência:** K0.

- Extrair responsabilidades coesas de roteamento, pós-spawn e diagnóstico para arquivos por domínio de ação.
- Reduzir **hookrun.go** e **doctor_hooks.go** para no máximo 500 linhas cada.
- Não alterar respostas, decisões, arquivos de estado ou comportamento do hook.

**Sensor computacional:** **make check** e diff sem mudanças de comportamento. **Sensor inferencial:** verificar nomes e limites de responsabilidade.

**PR:** refactor independente, sem documentação operacional.

## K2 — Batimento e doctor — 5 SP

**Dono:** Backend Platform. **Dependência:** K1. **QA:** teste e verificação independentes, 3 SP, na mesma entrega.

- Registrar cada invocação no runner compartilhado, inclusive quando o payload não gera decisão.
- Usar armazenamento limitado, rotação ou snapshot equivalente; validar o tamanho e os caracteres do nome da ferramenta.
- Guardar apenas horário UTC, harness registrado e nome de ferramenta validado. Excluir prompt, tarefa, caminho, IDs e payload.
- Preservar fail-open, latência limitada, permissões restritas e a saída esperada pelo harness.
- Fazer o doctor diferenciar hook configurado, chamada observada e decisão de roteamento; mostrar especificamente o **spawn_run** mais recente.
- Cobrir chamada válida, outro harness, payload inválido, falha de escrita, privacidade e estados ausente/recente/antigo no doctor.

**Aceite:** testes determinísticos passam sem **kiro-cli** real; a falha de persistência não bloqueia nem reescreve o hook; CI local e **make check** passam.

**PR:** comportamento e respectivos testes juntos. QA altera apenas arquivos de teste e entrega evidência.

## K3 — Guia KiroCrew — 1 SP

**Dono:** root. **Dependência:** K2.

- Atualizar **docs/guides/kirocrew-routing-guide.md** com configuração, interpretação dos sinais do doctor e a diferença entre batimento, decisão e execução do subagente.

**Aceite:** o guia não promete reescrita de modelo e não trata configuração como prova de execução.

**PR:** docs-only, após o comportamento estar disponível.

## K4 — Spawn real e comparação do modelo — 2 SP

**Dono:** root. **Dependência:** K2; discovery desligada; capacidade de iniciar um subagente disponível.

- Pedir explicitamente uma única chamada **spawn_run** no KiroCrew para comparar, em leitura, o setup, o runner e o doctor; não usar o pipeline nativo nem alterar arquivos.
- Conferir batimento de **spawn_run**, evento com **requested_model**/**final_model** e o **resolved_model** nativo.
- Não alterar sandbox nem outra configuração de segurança nesta tarefa. Se o ambiente bloquear a prova, registrar o motivo e manter status aberto.

**Aceite:** **resolved_model** corresponde ao **final_model**. Um teste unitário ou uma configuração versionada não substitui essa evidência.

## K5 — Fechamento com evidência — 1 SP

**Dono:** root. **Dependência:** K4.

- Atualizar estado, evidência e pendências no spike e roadmap.
- Se a prova real não for concluída, registrar o bloqueio sem declarar sucesso.

**PR:** docs-only, depois da execução real.

## Revisão de risco

- **AppSec, 2 SP:** revisar minimização de dados, rótulo de ferramenta, limites de armazenamento, permissões, latência e fail-open antes do merge de K2.
- **QA, 3 SP:** propor e executar os cenários listados em K2; retornar PASS/BLOCK com comandos e resultados. A autorização para QA veio do pedido “força total”.

## Fora de escopo

Alterar sandbox, rodar descoberta real, chamar comandos de login, publicar release, modificar credenciais ou limpar sessões e arquivos locais.

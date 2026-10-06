# 10 · Candidato em shadow

O hook de produção continua em `core.Route`. O classificador softmax da v2 não escolhe o modelo do subagente.

`harness-downshift` by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/harness-downshift

## O que muda no dia a dia

Com `DOWNSHIFT_SHADOW_WEIGHTS` apontando para um `candidate.json` absoluto, cada decisão do hook grava, no mesmo ID de feedback, a previsão do candidato: probabilidades, tier cru, tier depois da política de segurança, piso e o SHA-256 do artefato. A resposta do hook não muda. Um candidato inválido também não muda.

`success`, `retry` e `failed` continuam sendo feedback de qualidade. Só `--required-tier` vira rótulo de treino. Um sucesso antigo que ganhou tier mínimo por inferência precisa ser revisto antes de entrar em treino.

```bash
export DOWNSHIFT_SHADOW_WEIGHTS=/caminho/absoluto/candidate.json
downshift feedback <id> success
downshift feedback <id> retry --retry-tier=FRONTIER --required-tier=FRONTIER
downshift shadow-report
```

O procedimento completo, a leitura do relatório e o que ainda não existe (modelo neural/ONNX) estão em [`docs/CLASSIFIER-SHADOW.md`](../CLASSIFIER-SHADOW.md).

## O que isto não é

- Não promove a v2 para o hook.
- Não executa o modelo que o candidato teria escolhido.
- Não grava texto da tarefa, caminho do artefato nem mensagem de erro do backend.
- Não treina sozinho e não se ativa sozinho.

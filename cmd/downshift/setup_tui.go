// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/tiagovilasboas/downshift/internal/gateway/ninerouter"
	"github.com/tiagovilasboas/downshift/internal/gateway/omniroute"
)

// setupTUI presents an interactive terminal UI wizard for onboarding and
// configuring AI coding harnesses with Downshift.
func setupTUI(o setupOptions) int {
	renderTUIHeader(o.Out)

	targets, code := setupSelect(o)
	if code != 0 {
		return code
	}
	if len(targets) == 0 {
		fmt.Fprintln(o.Out, "No supported AI harnesses detected on this machine.")
		fmt.Fprintln(o.Out, "Supported harnesses: Claude Code, Cursor, OpenCode, OpenAI Codex, Google Antigravity, Kiro, KiroCrew.")
		return 0
	}

	renderTUIHarnesses(o.Out, targets)

	if o.In == nil {
		o.Yes = true
		return setupRun(o)
	}

	reader := bufio.NewReader(o.In)

	// Step 1: Configure or customize
	fmt.Fprint(o.Out, "Etapa [1/3] • Configurar os ambientes detectados com os defaults recomendados? [S/n/c] (S=Sim, n=Sair, c=Customizar) ")
	ans, _ := reader.ReadString('\n')
	ans = strings.TrimSpace(strings.ToLower(ans))

	if ans == "n" || ans == "nao" || ans == "não" || ans == "no" {
		fmt.Fprintln(o.Out, "Setup aborted by user.")
		return 0
	}

	if ans == "c" || ans == "customizar" {
		promptPreset(o.Out, reader, &o)
	}

	// Step 2: Gateway Companions
	probeAndRenderGateways(o.Out)

	// Step 3: Confirmation
	fmt.Fprint(o.Out, "Etapa [3/3] • Aplicar configurações agora com backup automático? [S/n] ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm != "" && confirm != "s" && confirm != "sim" && confirm != "y" && confirm != "yes" {
		fmt.Fprintln(o.Out, "Setup aborted by user.")
		return 0
	}

	o.Yes = true
	runCode := setupRun(o)
	if runCode == 0 && !o.DryRun {
		renderTUISuccess(o.Out)
	}
	return runCode
}

func renderTUIHeader(out io.Writer) {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)
	fmt.Fprintln(out, titleStyle.Render("⚡ Downshift Setup Wizard — Intelligent Harness Onboarding"))
}

func renderTUIHarnesses(out io.Writer, targets []setupTarget) {
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		MarginBottom(1)

	highlightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Bold(true)

	subtleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	var sb strings.Builder
	sb.WriteString("Ambientes de IA detectados nesta máquina (ordenados por relevância):\n\n")
	for i, t := range targets {
		sb.WriteString(fmt.Sprintf("  [%d] %-14s ~/%s\n", i+1, highlightStyle.Render(t.Name), t.ConfigDir))
	}
	sb.WriteString("\n" + subtleStyle.Render("Downshift aplicará governança de modelos, preservação de tokens e compactor shell."))
	fmt.Fprintln(out, cardStyle.Render(sb.String()))
}

func promptPreset(out io.Writer, reader *bufio.Reader, o *setupOptions) {
	fmt.Fprintln(out, "\nEtapa [2/3] • Escolha a postura de governança (Policy Preset):")
	fmt.Fprintln(out, "  [1] Equilibrado (Recomendado) — Trivial downshifts to small/flash; complex retains frontier.")
	fmt.Fprintln(out, "  [2] Economia Máxima (Aggressive) — Maximize cost reductions across subagent runs.")
	fmt.Fprintln(out, "  [3] Conservador (Safe) — Hold requested models whenever doubt exists.")
	fmt.Fprint(out, "Selecione o preset [1-3] (padrão: 1): ")
	p, _ := reader.ReadString('\n')
	p = strings.TrimSpace(p)
	switch p {
	case "2":
		fmt.Fprintln(out, "Preset selecionado: Economia Máxima (Aggressive)")
	case "3":
		fmt.Fprintln(out, "Preset selecionado: Conservador (Safe)")
	default:
		fmt.Fprintln(out, "Preset selecionado: Equilibrado (Balanced)")
	}
	fmt.Fprintln(out)
}

func probeAndRenderGateways(out io.Writer) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	omni := omniroute.New(150 * time.Millisecond)
	nine := ninerouter.New(150 * time.Millisecond)

	omniOK, _ := omni.CheckHealth(ctx, omni.DefaultEndpoint())
	nineOK, _ := nine.CheckHealth(ctx, nine.DefaultEndpoint())

	var detected []string
	if omniOK {
		detected = append(detected, fmt.Sprintf("OmniRoute (%s)", omni.DefaultEndpoint()))
	}
	if nineOK {
		detected = append(detected, fmt.Sprintf("9router (%s)", nine.DefaultEndpoint()))
	}

	if len(detected) > 0 {
		fmt.Fprintf(out, "✔ Gateways companheiros ativos detectados: %s\n", strings.Join(detected, ", "))
	} else {
		fmt.Fprintln(out, "• Nenhum gateway companheiro local ativo (OmniRoute/9router) detectado. Prosseguindo.")
	}
	fmt.Fprintln(out)
}

func renderTUISuccess(out io.Writer) {
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(0, 1).
		MarginTop(1)

	boldStyle := lipgloss.NewStyle().Bold(true)
	msg := fmt.Sprintf("%s\n\n%s\n  • %s\n  • %s\n  • %s",
		boldStyle.Render("🎉 Configuração concluída com sucesso!"),
		"Próximos passos:",
		"Abra uma nova janela ou sessão do seu harness (Claude Code, Cursor, OpenCode, Codex, Antigravity, Kiro).",
		"Execute 'downshift doctor' a qualquer momento para verificar a saúde das conexões.",
		"Execute 'downshift stats' para acompanhar sua economia de tokens em tempo real.",
	)
	fmt.Fprintln(out, cardStyle.Render(msg))
}

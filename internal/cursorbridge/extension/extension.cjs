// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
'use strict';
const vscode = require('vscode');
const fs = require('node:fs/promises');
const path = require('node:path');
const crypto = require('node:crypto');
const {decodeUsage, decodeModels, MAX_BYTES} = require('./decoder.cjs');

class EmptyRequest { toBinary() { return new Uint8Array(); } }
class UsageResponse {
  static fromBinary(bytes) {
    if (bytes.byteLength > MAX_BYTES) throw new Error('Response exceeds 1MiB');
    return decodeUsage(bytes);
  }
}
class ModelsRequest {
  // Matches stable subscription picker flags: exclude_max_named_models,
  // use_model_parameters, use_react_model_picker. No user-added/BYOK models.
  toBinary() { return Uint8Array.from([24, 1, 40, 1, 88, 1]); }
}
class ModelsResponse {
  static fromBinary(bytes) { return decodeModels(bytes); }
}
const modelsService = Object.freeze({typeName: 'aiserver.v1.AiService'});
const modelsMethod = Object.freeze({name: 'AvailableModels', I: ModelsRequest, O: ModelsResponse, kind: 0});
const service = Object.freeze({typeName: 'aiserver.v1.DashboardService'});
const method = Object.freeze({name: 'GetCurrentPeriodUsage', I: EmptyRequest, O: UsageResponse, kind: 0});
let running = false;

async function collect(context) {
  if (running) return;
  running = true;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 10000);
  let temporary;
  try {
    if (!vscode.workspace.isTrusted) throw new Error('A trusted workspace is required');
    const transport = vscode.cursor?.connectTransport;
    if (!transport || typeof transport.unary !== 'function') throw new Error('Native Cursor transport is unavailable');
    // The host owns authentication. Empty request and headers; no token API,
    // cookie, Keychain, storage database, network client or browser injection.
    const response = await transport.unary(service, method, controller.signal, 10000, {}, {});
    const models = await transport.unary(modelsService, modelsMethod, controller.signal, 10000, {}, {});
    const payload = {...response.message, available_models: models.message, available_models_complete: true};
    const target = process.env.DOWNSHIFT_CURSOR_NATIVE_FILE ||
      path.join(context.globalStorageUri.fsPath, 'cursor-usage.json');
    await fs.mkdir(path.dirname(target), {recursive: true, mode: 0o700});
    temporary = target + '.' + crypto.randomBytes(8).toString('hex') + '.tmp';
    await fs.writeFile(temporary, JSON.stringify(payload) + '\n', {mode: 0o600, flag: 'wx'});
    await fs.rename(temporary, target); temporary = undefined;
    await vscode.window.showInformationMessage('Current Cursor quota exported locally: ' + target);
    return target;
  } catch {
    // Transport errors may carry account metadata. Never expose or persist them.
    await vscode.window.showWarningMessage('Cursor quota export failed. No new quota was stored; check native Cursor login and bridge compatibility.');
  } finally {
    clearTimeout(timer);
    if (temporary) await fs.unlink(temporary).catch(() => {});
    running = false;
  }
}
function activate(context) {
  context.subscriptions.push(vscode.commands.registerCommand('downshift.cursorQuota.export', () => collect(context)));
}
module.exports = {activate};

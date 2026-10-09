# Downshift Cursor Quota Bridge (experimental)

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

This experimental extension attempts to export current quota and available model
IDs through Cursor's native authenticated transport. It makes no model inference request,
reads no credential store, and does not start a CLI login. Cursor's internal API
can change; installation alone does not prove collection works.

The only RPCs are `DashboardService/GetCurrentPeriodUsage` and
`AiService/AvailableModels`; authentication stays inside the host transport.

**Runtime blocker on Cursor 3.24.9:** the `cursor` proposal is restricted to
built-in extensions. The external extension installed and its command appeared,
but a fresh host with the exact proposal flag still failed at `transport`, before
either RPC. No native export was produced. This is not a working quota collector
on that version; the proposal flag does not override the built-in restriction.
Do not modify the vendor application or impersonate a built-in extension.

For a future compatible host or reproduction of this limitation, package and
install explicitly from the Downshift repository root:

```bash
python3 internal/cursorbridge/package.py /tmp/downshift-cursor-quota-bridge.vsix
cursor --install-extension /tmp/downshift-cursor-quota-bridge.vsix
cursor --new-window --enable-proposed-api tiagovilasboas.downshift-cursor-quota-bridge /path/to/trusted/workspace
```

The manifest declares the internal `cursor` API proposal. A compatible host may
also require the exact-ID opt-in flag; Cursor 3.24.9 additionally enforces the
built-in-only restriction above. Where available, that proposal exposes a broad
internal API, while this code uses only the two read-only RPCs named above. Do not
enable proposals globally or copy the flag to persistent launch configuration.
Vendor versions and API availability must be verified again after updates.

The packager refuses to overwrite an existing output file. Choose a new path
for a later package. No registry packages or publishing credentials are needed.

In a compatible trusted Cursor editor extension host, invoke **Downshift: Export Cursor
Quota (Experimental)** from the Command Palette. The export defaults to
`<Cursor User/globalStorage>/tiagovilasboas.downshift-cursor-quota-bridge/cursor-usage.json`.
If `DOWNSHIFT_CURSOR_NATIVE_FILE` is supplied to the extension host, that path is
used instead. Supply the same path to the Downshift hook separately; a child
extension cannot change the hook's inherited environment.

Each invocation collects once. There is no background collector: the five-minute
observation expires until another native capture succeeds. Keep model availability
and quota fresh together. Do not replay an old export with a new timestamp.
Verify both pool percentages, exact `auto_bucket_models`,
`available_models_complete`, and `observed_at` without printing account metadata.
Display labels, a success notification, or installed-extension listing do not
prove a rewrite was honored by Cursor's child executor.

Remove the optional extension with:

```bash
cursor --uninstall-extension tiagovilasboas.downshift-cursor-quota-bridge
```

Guia inferencial: native authentication, export cadence, and scope above.
Sensor computacional: decoder tests, quota/session gates, and real export
validation. Eixos: behaviour and architecture fitness. Continuous freshness is
an operator responsibility until a verified continuous collector exists.

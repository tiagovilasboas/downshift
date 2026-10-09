#!/usr/bin/env python3
# Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
"""Package the opt-in Cursor bridge locally, without registry dependencies."""

import json
from pathlib import Path
import sys
from xml.sax.saxutils import escape
from zipfile import ZIP_DEFLATED, ZipFile


def package(target: Path) -> None:
    root = Path(__file__).resolve().parent
    extension = root / "extension"
    metadata = json.loads((extension / "package.json").read_text())
    value = lambda key: escape(metadata[key], {'"': "&quot;"})
    manifest = f'''<?xml version="1.0" encoding="utf-8"?>
<PackageManifest Version="2.0.0" xmlns="http://schemas.microsoft.com/developer/vsx-schema/2011">
  <Metadata>
    <Identity Language="en-US" Id="{value('name')}" Version="{value('version')}" Publisher="{value('publisher')}" />
    <DisplayName>{value('displayName')}</DisplayName>
    <Description xml:space="preserve">{value('description')}</Description>
    <Properties>
      <Property Id="Microsoft.VisualStudio.Code.Engine" Value="{escape(metadata['engines']['vscode'])}" />
      <Property Id="Microsoft.VisualStudio.Code.ExtensionKind" Value="ui" />
    </Properties>
    <License>extension/LICENSE</License>
  </Metadata>
  <Installation><InstallationTarget Id="Microsoft.VisualStudio.Code" /></Installation>
  <Dependencies />
  <Assets>
    <Asset Type="Microsoft.VisualStudio.Code.Manifest" Path="extension/package.json" Addressable="true" />
    <Asset Type="Microsoft.VisualStudio.Services.Content.Details" Path="extension/README.md" Addressable="true" />
    <Asset Type="Microsoft.VisualStudio.Services.Content.License" Path="extension/LICENSE" Addressable="true" />
  </Assets>
</PackageManifest>
'''
    content_types = '''<?xml version="1.0" encoding="utf-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="json" ContentType="application/json" />
  <Default Extension="cjs" ContentType="application/javascript" />
  <Default Extension="md" ContentType="text/markdown" />
  <Default Extension="vsixmanifest" ContentType="text/xml" />
  <Default Extension="" ContentType="text/plain" />
</Types>
'''
    # Explicit files only: no environment, local exports, caches, or private state.
    files = {f"extension/{name}": extension / name for name in
             ("package.json", "extension.cjs", "decoder.cjs", "README.md")}
    files["extension/LICENSE"] = root.parent.parent / "LICENSE"
    files["extension/NOTICE"] = root.parent.parent / "NOTICE"
    with ZipFile(target, "x", compression=ZIP_DEFLATED) as archive:
        archive.writestr("extension.vsixmanifest", manifest)
        archive.writestr("[Content_Types].xml", content_types)
        for destination, source in files.items():
            archive.write(source, destination)
    print(target)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: python3 internal/cursorbridge/package.py OUTPUT.vsix")
    package(Path(sys.argv[1]).expanduser().resolve())

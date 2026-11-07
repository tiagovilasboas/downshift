#!/usr/bin/env python3
"""Launch one isolated Codex MCP evaluation run; scoring remains manual."""
import argparse
import hashlib
import json
import pathlib
import subprocess
import sys
import tempfile
import time

ROOT = pathlib.Path(__file__).resolve().parents[2]


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--case', required=True)
    p.add_argument('--model', required=True)
    p.add_argument('--arm', required=True, choices=['A', 'B'])
    p.add_argument('--repetition', required=True, type=int, choices=range(1, 4))
    p.add_argument('--instructions', type=pathlib.Path, required=True)
    p.add_argument('--fixture', type=pathlib.Path, required=True)
    p.add_argument('--output', type=pathlib.Path, required=True, help='new output directory; contains private transcripts')
    p.add_argument('--timeout', type=int, default=300)
    args = p.parse_args()
    try:
        manifest = json.loads((ROOT / 'docs/evaluation/mcp-instructions/cases.json').read_text())
        case = next(item for item in manifest['cases'] if item['id'] == args.case)
        instructions = args.instructions.resolve(strict=True)
        fixture = args.fixture.resolve(strict=True)
        args.output.mkdir(mode=0o700, parents=False, exist_ok=False)
        output = args.output.resolve()
        with tempfile.TemporaryDirectory(prefix='downshift-mcp-eval-') as work:
            pathlib.Path(work, 'answer.txt').write_text('ready')
            pathlib.Path(work, 'fixture_api.py').write_text('def public_entry():\n    return "ready"\n\ndef _private_helper():\n    return None\n')
            instruction_bytes, fixture_bytes = instructions.read_bytes(), fixture.read_bytes()
            instruction_snapshot = pathlib.Path(work, 'instructions.txt')
            fixture_snapshot = pathlib.Path(work, 'fixture.py')
            instruction_snapshot.write_bytes(instruction_bytes)
            fixture_snapshot.write_bytes(fixture_bytes)
            fixture_args = [str(fixture_snapshot), '--case', args.case, '--model', args.model, '--instructions', str(instruction_snapshot), '--log', str(output / 'calls.jsonl')]
            cmd = ['codex', 'exec', '--ignore-user-config', '--ephemeral', '--skip-git-repo-check',
                   '--sandbox', 'workspace-write', '--add-dir', str(output), '--json', '-C', work, '-m', args.model,
                   '-c', 'mcp_servers.downshift.command="python3"',
                   '-c', 'mcp_servers.downshift.args=' + json.dumps(fixture_args), case['prompt']]
            if args.case in {'C01', 'C02'}:
                cmd[2:2] = ['--enable', 'multi_agent']
            started = time.monotonic()
            with (output / 'events.jsonl').open('w', encoding='utf-8') as events, (output / 'stderr.log').open('w', encoding='utf-8') as errors:
                result = subprocess.run(cmd, stdout=events, stderr=errors, timeout=args.timeout)
            metadata = {'case': args.case, 'arm': args.arm, 'repetition': args.repetition, 'model': args.model, 'exit_code': result.returncode,
                        'elapsed_seconds': round(time.monotonic() - started, 3),
                        'instructions_sha256': hashlib.sha256(instruction_bytes).hexdigest(),
                        'fixture_sha256': hashlib.sha256(fixture_bytes).hexdigest(),
                        'scored': False, 'note': 'Inspect call and event logs; this is not an acceptance result.'}
            (output / 'run.json').write_text(json.dumps(metadata, indent=2) + '\n')
            return 0 if result.returncode == 0 else 1
    except (OSError, ValueError, KeyError, StopIteration, subprocess.TimeoutExpired) as error:
        print(f'launch failed: {error}', file=sys.stderr)
        return 2


if __name__ == '__main__':
    sys.exit(main())

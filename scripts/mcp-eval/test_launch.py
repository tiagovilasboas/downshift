import json
import os
import pathlib
import subprocess
import tempfile
import unittest


class LaunchTest(unittest.TestCase):
    def test_launch_records_unscored_run(self):
        launch = pathlib.Path(__file__).with_name('launch.py').resolve()
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            codex = root / 'codex'
            codex.write_text('#!/bin/sh\nprintf "{\\"type\\":\\"mock\\"}\\n"\n')
            codex.chmod(0o755)
            instructions = root / 'instructions.txt'
            instructions.write_text('fixture instructions')
            fixture = root / 'fixture.py'
            fixture.write_text('# placeholder')
            output = root / 'result'
            env = os.environ.copy()
            env['PATH'] = str(root) + os.pathsep + env['PATH']
            result = subprocess.run(['python3', str(launch), '--case', 'C02', '--model', 'test',
                                     '--arm', 'A', '--repetition', '1', '--instructions', str(instructions), '--fixture', str(fixture),
                                     '--output', str(output)], env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            metadata = json.loads((output / 'run.json').read_text())
            self.assertFalse(metadata['scored'])
            self.assertEqual(metadata['case'], 'C02')
            self.assertTrue((output / 'events.jsonl').exists())
            repeat = subprocess.run(['python3', str(launch), '--case', 'C02', '--model', 'test',
                                     '--arm', 'A', '--repetition', '1', '--instructions', str(instructions), '--fixture', str(fixture),
                                     '--output', str(output)], env=env, capture_output=True, text=True)
            self.assertEqual(repeat.returncode, 2)


if __name__ == '__main__':
    unittest.main()

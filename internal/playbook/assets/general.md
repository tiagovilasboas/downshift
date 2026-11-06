# General

- Read the repository's own rules first (CLAUDE.md, AGENTS.md, CONTRIBUTING.md). They win over these rules.
- Change only what the task asks for. Do not refactor, rename or reformat unrelated code.
- Keep each change small and self-contained, and leave the code in a state that builds.
- Verify before you report: run the project's own check or test command and report its real result.
- Say plainly what you did not verify. Never report a green result you did not see.
- Do not commit, push or open a pull request unless the task asks for it.
- Do not add secrets, tokens or personal data to code, logs or messages.
- Edit only the files your brief allows. If the change needs another file, stop and report it instead.
- Run every allowlisted command as `downshift compactor -- <command>`.
- Finish with the verification command's exit code, its compactor output and the diff. No greeting, no summary of intent.

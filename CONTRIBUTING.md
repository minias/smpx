# CONTRIBUTING

**English** | [한국어](CONTRIBUTING_ko.md)

## Branch strategy

- `main`: release branch (protected — no force push, CodeQL must pass)
- `dev`: integration branch
- Always branch off `dev` (`feature/*`, `fix/*`, `chore/*`, `docs/*`), verify, then open a PR into `dev`.

## Commits / PRs

- Prefix commit messages with `feat:`, `fix:`, `chore:`, `docs:` or `deps:`.
- Before opening a PR, make sure these pass:

```bash
go vet ./...
go test ./...
cd frontend && npm run build
```

- This is a public repository: never commit secrets (tokens, keys, passwords, certificates), personal data or local paths. CI runs a gitleaks scan on every PR.

## Security

Report vulnerabilities through [SECURITY.md](SECURITY.md), not as public issues.

## Code of Conduct

Everyone participating in this project is expected to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

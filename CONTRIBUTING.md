# CONTRIBUTING

## 브랜치 전략

- `main`: 배포 브랜치 (보호 규칙 적용, 강제 push 금지, CodeQL 통과 필요)
- `dev`: 통합 브랜치
- 작업은 항상 `dev` 에서 새 브랜치(`feature/*`, `fix/*`, `chore/*`)를 만들어 진행하고, 검증 후 PR 로 `dev` 에 병합합니다.

## 커밋 / PR

- 커밋 메시지는 `feat:`, `fix:`, `chore:`, `docs:`, `deps:` 접두어를 사용합니다.
- PR 전에 다음을 통과해야 합니다.

```bash
go vet ./...
go test ./...
cd frontend && npm run build
```

## 보안

취약점은 이슈가 아닌 [SECURITY.md](SECURITY.md) 의 절차로 제보해 주세요.

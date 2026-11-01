# CONTRIBUTING

[English](CONTRIBUTING.md) | **한국어**

## 브랜치 전략

- `main`: 배포 브랜치 (보호 규칙 적용, 강제 push 금지, CodeQL 통과 필요)
- `dev`: 통합 브랜치
- 작업은 항상 `dev` 에서 새 브랜치(`feature/*`, `fix/*`, `chore/*`, `docs/*`)를 만들어 진행하고, 검증 후 PR 로 `dev` 에 병합합니다.

## 커밋 / PR

- 커밋 메시지는 `feat:`, `fix:`, `chore:`, `docs:`, `deps:` 접두어를 사용합니다.
- PR 전에 다음을 통과해야 합니다.

```bash
go vet ./...
go test ./...
cd frontend && npm run build
```

- 공개 저장소입니다: 비밀 정보(토큰, 키, 암호, 인증서), 개인 정보, 로컬 경로를 커밋하지 마세요. 모든 PR 에서 gitleaks 검사가 실행됩니다.

## 보안

취약점은 이슈가 아닌 [SECURITY_ko.md](SECURITY_ko.md) 의 절차로 제보해 주세요.

## 행동 강령

이 프로젝트에 참여하는 모든 분은 [행동 강령](CODE_OF_CONDUCT_ko.md) 을 따라 주세요.

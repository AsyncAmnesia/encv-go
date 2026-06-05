# preview-gateway

Sandbox preview gateway. See `.trae/specs/unify-sandbox-preview-port/spec.md` for full spec.

Quick start:
```bash
pnpm install
pnpm build
pm2 start /workspace/ecosystem.config.cjs
curl http://localhost:16000/__gateway/health
```

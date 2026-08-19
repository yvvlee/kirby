# Kirby Web

这是 Kirby 的 React 19 和 TypeScript 6 管理前端。

## 环境

- Node.js 24.19.0
- npm 11.17.0

进入目录后使用 nvm 切换版本：

```bash
nvm use
npm ci --registry=https://registry.npmjs.org/
```

## 命令

```bash
npm run dev
npm run typecheck
npm run lint
npm test
npm run build
npm run test:e2e
```

开发服务器监听 `15173`。`/api` 默认代理到 `http://127.0.0.1:8000`。可通过 `KIRBY_DEV_API_TARGET` 修改后端地址。

第三方类型兼容性见 `COMPATIBILITY.md`。

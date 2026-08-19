# 前端依赖兼容性记录

## React 19 与 Formily 类型声明

当前组合：

- React 19.2.8
- TypeScript 6.0.3
- Formily 2.3.7
- @formily/antd-v5 1.2.4
- Ant Design 5.29.3

Formily 和部分 Ant Design 间接依赖的声明文件仍引用 React 18 的全局 `JSX`、`ReactFragment` 和已经变化的 `rc-*` 类型导出。应用可以在 React 19 下渲染和交互，但 TypeScript 6 检查 `node_modules` 时会失败。

`@formily/grid@2.3.7` 还把 TypeScript 对等依赖范围声明为 `4.x || 5.x`。这是当前依赖树中唯一的 `npm ls` 异常。`scripts/license-check.sh` 只允许这个精确版本组合，并强制运行 Formily 兼容测试。版本或异常内容变化会使检查失败。

因此 `tsconfig.app.json` 和 `tsconfig.node.json` 使用 `skipLibCheck: true`。该设置只跳过依赖包声明文件的相互检查。`src` 内代码仍启用 `strict`、`noUncheckedIndexedAccess` 和 `exactOptionalPropertyTypes`。

兼容性不能只由类型配置保证。`src/App.test.tsx`、SchemaForm 测试和真实后端 E2E 覆盖值写入、值读取、禁用状态、数组操作、日期序列化和文件字段。

## 项目规范
- 代码务必简洁、低耦合、模块化，以便后续的升级维护
- 完成一个模块后必须要有并通过测试用例
- react 组件必须使用 function 方式，不使用箭头函数方式声明函数，明确参数与返回值类型，必要时可在 `src/types` 中存放公共部分
- 用 `xxx.module.scss` 方式来使用 CSS
- 前端部分目录、文件名称统一用大驼峰，涉及 `utils` 或者公共 `components` 部分，要统一导出在 `index.ts` 或 `index.tsx` 中再使用，例如：
```javascript
// utils/string.ts
export function foo() {
  // ...
}

// utils/index.ts
import { foo } from './string'

export { foo }
```
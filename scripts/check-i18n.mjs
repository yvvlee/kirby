import ts from '../web/node_modules/typescript/lib/typescript.js'
import { readFileSync, readdirSync } from 'node:fs'
import { resolve, relative } from 'node:path'

const root = resolve(import.meta.dirname, '..')
const sources = resolve(root, 'web/src')
function files(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = resolve(directory, entry.name)
    return entry.isDirectory() ? files(path) : [path]
  })
}
function parse(path) {
  return ts.createSourceFile(path, readFileSync(path, 'utf8'), ts.ScriptTarget.Latest, true, path.endsWith('tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS)
}
const keys = new Set()
const translations = new Map()
const problems = []
for (const filename of ['index.ts', 'ui.ts', 'config-center.ts']) {
  const file = parse(resolve(sources, 'i18n', filename))
  function visit(node) {
    if (ts.isPropertyAssignment(node) && ts.isStringLiteral(node.name) && ts.isStringLiteral(node.initializer)) {
      const key = node.name.text
      const value = node.initializer.text
      if (!value.trim()) problems.push(`Empty translation: ${key}`)
      const placeholders = (text) => [...text.matchAll(/\{\{(\w+)\}\}/g)].map((match) => match[1]).sort().join(',')
      if (placeholders(key) !== placeholders(value)) problems.push(`Mismatched placeholders: ${key}`)
      if (translations.has(key) && translations.get(key) !== value) problems.push(`Conflicting translations: ${key}`)
      keys.add(key)
      translations.set(key, value)
    }
    ts.forEachChild(node, visit)
  }
  visit(file)
}
const missing = []
for (const path of files(sources).filter((path) => /\.tsx?$/.test(path) && !path.includes('.test.') && !path.includes('/i18n/'))) {
  const file = parse(path)
  function visit(node) {
    if (ts.isJsxText(node) && /[\p{Script=Han}]/u.test(node.text)) missing.push(`${relative(root, path)}: untranslated JSX ${node.text.trim()}`)
    if (ts.isCallExpression(node) && ['t', 'localizedError'].includes(node.expression.getText(file)) && node.arguments.length) {
      const argument = node.arguments[0]
      const candidates = ts.isConditionalExpression(argument) ? [argument.whenTrue, argument.whenFalse] : [argument]
      for (const candidate of candidates.filter(ts.isStringLiteral)) {
        if (!keys.has(candidate.text)) missing.push(`${relative(root, path)}: ${candidate.text}`)
      }
    }
    ts.forEachChild(node, visit)
  }
  visit(file)
}
if (missing.length || problems.length) {
  console.error([...problems, ...missing].join('\n'))
  process.exitCode = 1
} else console.log(`PASS: all static translation calls have catalog entries (${keys.size} messages)`)

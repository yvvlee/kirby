# Internationalization

Kirby supports Simplified Chinese (`zh-CN`) and English (`en-US`). New browser
sessions default to Simplified Chinese. The language selector is available on
public account pages and in the application header. The explicit choice is
stored in browser local storage under `kirby.locale` and survives reloads.
Switching language does not reload the page or discard unsaved form inputs.

Application text and Ant Design controls use the selected language. Project
names, configuration values, usernames, and other user content are never
translated. Stable API field names, enum values and error reason codes do not
change with the display language.

## Frontend development

Use `useTranslation()` from `@/i18n` in React components so they subscribe to
language changes. Use `t()` in non-React helpers, evaluated when the helper runs,
not in a module-level translated constant. Chinese source messages are catalog
keys; English translations live in `web/src/i18n/`. Keep dynamic values separate
with named placeholders, for example `t('环境：{{name}}', { name })`.

Do not translate stored data or construct keys by concatenating user input.
Preserve the application's `autoComplete="off"` behavior when adding inputs.

Use `localizedError()` from `@/i18n/errors` when raising application errors.
It retains the catalog key and parameters so an existing error can be rendered
in the current language. Store the error object in component state, rather than
storing its already translated message. Server responses use the language of
the request that produced them.

Monaco loads separate Chinese and English language resources. Switching the
interface language recreates only the editor instance and keeps its current
unsaved text. Configuration data and programming-language identifiers are not
translated.

Run `node scripts/check-i18n.mjs` to check static translation calls against the
catalogs, then run frontend lint, tests and the production build. Catalog checks
supplement manual review of pages, dialogs, validation and accessibility labels;
they do not prove that every visible message has been converted.

## API requests

The browser sends its active locale in `Accept-Language`, including refresh
requests. API clients can choose a supported language through the same header.
See the server's localization tests for language negotiation and stable error
codes. Configuration payloads remain byte-for-byte business data regardless of
request language.

The command-line tool uses `LANG` for its help, administrator prompts and
success messages. For example, `LANG=en_US.UTF-8 ./kirby --help` uses English;
`LANG=zh_CN.UTF-8 ./kirby --help` uses Simplified Chinese. Command names, flag
names, operational logs and low-level configuration/IO diagnostics stay stable.

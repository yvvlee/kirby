import { t, type TranslationParams } from './index'

/**
 * Keeps application error text translatable after it has been raised.
 * Parameters: text is a catalog key; params holds untouched values; ErrorClass preserves the error category.
 * Returns: an error whose message uses the currently selected language.
 * Throws: none.
 */
export function localizedError(
  text: string,
  params: TranslationParams = {},
  ErrorClass: ErrorConstructor = Error,
): Error {
  const error = new ErrorClass()
  Object.defineProperty(error, 'message', {
    configurable: true,
    get: () => t(text, params),
  })
  return error
}

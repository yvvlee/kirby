type MonacoWorkerEnvironment = {
  getWorker: (_moduleId: string, label: string) => Worker
}

export type MonacoWorkerFactory = new () => Worker

/**
 * Configures workers for the locale-specific Monaco module graph.
 *
 * Parameters:
 * - EditorWorker: Worker constructor for editor language services.
 * - JsonWorker: Worker constructor for JSON language services.
 *
 * Returns: Nothing.
 * Throws: Nothing; worker construction errors are reported when Monaco requests a worker.
 */
export function configureMonacoWorkers(
  EditorWorker: MonacoWorkerFactory,
  JsonWorker: MonacoWorkerFactory,
): void {
  ;(globalThis as typeof globalThis & { MonacoEnvironment: MonacoWorkerEnvironment })
    .MonacoEnvironment = {
    getWorker: (_moduleId, label) =>
      label === 'json' ? new JsonWorker() : new EditorWorker(),
  }
}

import { connect } from '@formily/react'

import type { Identifier } from '@/api/types'
import { positiveIdentifier } from '@/api/environment-resource'
import FileUpload, { type FileUploadProps } from './FileUpload'

type ScopedProps = Omit<FileUploadProps, 'environmentId' | 'projectId'>

export function createScopedFileUpload(
  environmentId: Identifier,
  projectId: Identifier,
) {
  const environment = positiveIdentifier(environmentId, 'environmentId')
  const project = positiveIdentifier(projectId, 'projectId')
  const ScopedFileUpload = (props: ScopedProps) => (
    <FileUpload {...props} environmentId={environment} projectId={project} />
  )
  ScopedFileUpload.displayName = `ScopedFileUpload_${environment}_${project}`
  return connect(ScopedFileUpload)
}

<template>
  <div
    ref="root"
    class="diff-editor"
  />
</template>

<script>
import * as monaco from 'monaco-editor/editor/editor.api'
import 'monaco-editor/language/json/monaco.contribution'

import { configureMonacoWorkers } from '@/components/MonacoEditor/environment.js'
import { formatDiffValue } from './format.js'

configureMonacoWorkers()

export default {
  name: 'DiffEditor',
  props: {
    leftValue: {
      type: [Object, Array, String, Number, Boolean],
      default: null,
    },
    rightValue: {
      type: [Object, Array, String, Number, Boolean],
      default: null,
    },
  },
  data() {
    return {
      diffEditor: null,
      originalModel: null,
      modifiedModel: null,
    }
  },
  watch: {
    leftValue: {
      deep: true,
      handler(value) {
        this.originalModel?.setValue(formatDiffValue(value))
      },
    },
    rightValue: {
      deep: true,
      handler(value) {
        this.modifiedModel?.setValue(formatDiffValue(value))
      },
    },
  },
  mounted() {
    this.diffEditor = monaco.editor.createDiffEditor(this.$refs.root, {
      automaticLayout: true,
      wordWrap: 'on',
      diffWordWrap: 'on',
      folding: true,
      theme: 'vs-light',
      readOnly: true,
      bracketPairColorization: { enabled: true },
    })
    this.originalModel = monaco.editor.createModel(
      formatDiffValue(this.leftValue),
      'json',
    )
    this.modifiedModel = monaco.editor.createModel(
      formatDiffValue(this.rightValue),
      'json',
    )
    this.diffEditor.setModel({
      original: this.originalModel,
      modified: this.modifiedModel,
    })
  },
  beforeDestroy() {
    this.diffEditor?.setModel(null)
    this.originalModel?.dispose()
    this.modifiedModel?.dispose()
    this.diffEditor?.dispose()
  },
}
</script>

<style scoped>
.diff-editor {
  width: 100%;
  min-height: 240px;
  max-height: 480px;
}
</style>

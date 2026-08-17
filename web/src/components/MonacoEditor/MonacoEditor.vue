<template>
  <div
    ref="editorContainer"
    class="monaco-editor-container"
  />
</template>

<script>
import * as monaco from 'monaco-editor/editor/editor.api'
import 'monaco-editor/language/json/monaco.contribution'

import { configureMonacoWorkers } from './environment.js'

configureMonacoWorkers()

export default {
  name: 'MonacoEditor',
  props: {
    value: {
      type: String,
      default: '',
    },
    disabled: {
      type: Boolean,
      default: false,
    },
    language: {
      type: String,
      default: 'json',
    },
  },
  data() {
    return {
      editor: null,
      changeSubscription: null,
    }
  },
  watch: {
    value(newValue) {
      this.setValue(newValue)
    },
    disabled(newValue) {
      this.editor?.updateOptions({ readOnly: newValue })
    },
  },
  mounted() {
    this.editor = monaco.editor.create(this.$refs.editorContainer, {
      value: this.value,
      language: this.language,
      automaticLayout: true,
      minimap: { enabled: false },
      readOnly: this.disabled,
    })
    this.changeSubscription = this.editor.onDidChangeModelContent(() => {
      const nextValue = this.editor.getValue()
      if (nextValue !== this.value) {
        this.$emit('input', nextValue)
        this.$emit('change', nextValue)
      }
    })
  },
  beforeDestroy() {
    this.changeSubscription?.dispose()
    this.editor?.dispose()
  },
  methods: {
    getValue() {
      return this.editor ? this.editor.getValue() : this.value
    },
    setValue(newValue) {
      if (typeof newValue !== 'string') {
        throw new TypeError('MonacoEditor 的 value 必须是字符串')
      }
      if (this.editor && this.editor.getValue() !== newValue) {
        this.editor.setValue(newValue)
      }
    },
  },
}
</script>

<style scoped>
.monaco-editor-container {
  width: 100%;
  min-height: 240px;
}
</style>

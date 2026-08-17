<template>
  <span
    class="environment-tag"
    :style="tagStyle"
    :title="description"
  >
    {{ label }}
  </span>
</template>

<script>
function hash(value) {
  let result = 0
  for (const character of value) {
    result = (result * 31 + character.codePointAt(0)) % 360
  }
  return result
}

export default {
  name: 'EnvironmentTag',

  props: {
    environment: {
      type: Object,
      default: null,
    },
  },

  computed: {
    label() {
      return this.environment?.name || '未选择环境'
    },
    description() {
      return this.environment?.description || this.label
    },
    tagStyle() {
      const identity = this.environment?.key || this.label
      const hue = hash(identity)
      return {
        backgroundColor: `hsl(${hue} 70% 92%)`,
        borderColor: `hsl(${hue} 55% 72%)`,
        color: `hsl(${hue} 55% 28%)`,
      }
    },
  },
}
</script>

<style scoped>
.environment-tag {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 2px 9px;
  border: 1px solid;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  line-height: 18px;
  white-space: nowrap;
}
</style>

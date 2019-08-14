<template>
    <v-snackbar v-model="visible"
                :color="type"
                multi-line
                :timeout="timeout"
                left
    >
        {{ message(type) }}
        <v-btn dark text @click="close">
            Close
        </v-btn>
    </v-snackbar>
</template>

<script>

  export default {
    name: "Result",
    props: {
      value: {
        type: Boolean
      },
      type: {
        type: String,
        default: 'success'
      },
      timeout: {
        type: Number,
        default: 2000
      }
    },
    data() {
      return {
        visible: this.value
      }
    },
    methods: {
      message(type) {
        if (type === 'success') {
          return this.$t('settings.UPDATE_CONFIG_SUCCESS')
        } else if (type === 'error') {
          return this.$t('settings.UPDATE_CONFIG_ERROR')
        }
      },
      close() {
        this.visible = false
      }
    },
    watch: {
      visible(v) {
        this.$emit('input', v)
      },
      value(v) {
        this.visible = v
      }
    }
  }
</script>


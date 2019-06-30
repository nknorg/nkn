<template>
  <v-snackbar v-if="inBoundCount !== 0"
              v-model="visible"
              color="warning"
              multi-line
              :timeout="timeout"
              right
  >
    {{ inBoundCount }}
    <v-btn dark flat @click="close">
      Close
    </v-btn>
  </v-snackbar>
</template>

<script>
  import {mapGetters} from 'vuex'

  export default {
    name: "WatchOutBoundCount",
    computed: {
      ...mapGetters({
        inBoundCount: 'node/inBoundCount'
      }),
    },
    data() {
      return {
        visible: true,
        mode: '',
        timeout: 0,
        text: 'Hello, I\'m a snackbar'
      }
    },
    methods: {
      close() {
        this.visible = false
        setTimeout(() => {
          this.visible = true
        }, 10000)
      }
    }
  }
</script>

<style scoped>

</style>

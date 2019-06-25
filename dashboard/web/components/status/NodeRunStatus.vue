<template>
  <v-badge color="transparent" class="breathe mr-4">
    <template v-slot:badge>
      <v-icon dark small color="green accent-4">lens</v-icon>
    </template>
    <span>Node status: starting</span>
  </v-badge>
</template>

<script>
  export default {
    name: "NodeStatus"
  }
</script>

<style scoped>

</style>

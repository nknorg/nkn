<template>
  <v-layout flex row wrap>
    <v-flex>
      <Neighbors></Neighbors>
    </v-flex>
  </v-layout>

</template>

<script>
  import Neighbors from '~/components/widget/Neighbors.vue'

  export default {
    components: {
      Neighbors
    },

  }
</script>

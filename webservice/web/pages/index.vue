<template>
  <v-layout flex row wrap>
    <v-flex>
      <LastMiningRewards></LastMiningRewards>
    </v-flex>
    <v-flex>
      <Neighbors></Neighbors>
    </v-flex>
  </v-layout>

</template>

<script>
  import LastMiningRewards from '~/components/widget/LastMiningRewards.vue'
  import Neighbors from '~/components/widget/Neighbors.vue'

  export default {
    components: {
      LastMiningRewards,
      Neighbors
    },

    async mounted() {

    }
  }
</script>

<template>
    <v-container fluid grid-list-lg>
        <v-layout flex align-center justify-center fill-height>
            <v-progress-circular
                    :size="70"
                    :width="7"
                    color="grey"
                    indeterminate
            ></v-progress-circular>
        </v-layout>
    </v-container>
</template>

<script>
  import {ServiceStatusEnum} from '~/helpers/consts'
  import {mapState} from 'vuex'

  export default {
    layout: 'null',
    name: "loading",
    computed: {
      ...mapState({
        serviceStatus: state => state.serviceStatus
      })
    },
    data() {
      return {}
    },
    created() {
      this.loadPage()
    },
    watch: {
      serviceStatus(val) {
        this.loadPage()
      }
    },
    methods: {
      loadPage() {
        // let currentPage = this.$route.name
        if ((this.serviceStatus & ServiceStatusEnum.SERVICE_STATUS_NO_PASSWORD) > 0) {
          this.$router.push(this.localePath('wallet-open'))
        } else if ((this.serviceStatus & ServiceStatusEnum.SERVICE_STATUS_NO_WALLET_FILE) > 0) {
          this.$router.push(this.localePath('wallet-create'))
        } else if ((this.serviceStatus & ServiceStatusEnum.SERVICE_STATUS_RUNNING) > 0){
          this.$router.push(this.localePath('index'))
        }

      }
    }
  }
</script>


<template>
    <v-badge color="transparent" class="breathe mr-4">
        <template v-slot:badge>
            <v-icon dark small :color="getStatusColor(nodeStatus.syncState)">fa-circle</v-icon>
        </template>
        <span> {{getStatus( nodeStatus.syncState)}}</span>
    </v-badge>
</template>

<script>
  import {mapState} from 'vuex'
  import {ServiceStatusEnum} from '~/helpers/consts'

  export default {
    name: "NodeStatus",
    props: {
      nodeStatus: {
        type: Object,
        required: true
      }
    },
    data: () => ({
      color: {
        'DEFAULT': 'grey',
        'WAIT_FOR_SYNCING': 'yellow accent-2',
        'SYNC_STARTED': 'green accent-4',
        'SYNC_FINISHED': 'green accent-4',
        'PERSIST_FINISHED': 'green accent-4'
      }
    }),
    computed: {
      ...mapState({
        serviceStatus: state => state.serviceStatus
      })
    },
    mounted() {

    },
    methods: {
      // get node status
      getStatus(stateStr) {
        const statusEnum = {
          'WAIT_FOR_SYNCING': this.$t('node.state.WAIT_FOR_SYNCING'),
          'SYNC_STARTED': this.$t('node.state.SYNC_STARTED'),
          'SYNC_FINISHED': this.$t('node.state.SYNC_FINISHED'),
          'PERSIST_FINISHED': this.$t('node.state.PERSIST_FINISHED')
        }

        return statusEnum[stateStr] || this.getServiceStatus(this.serviceStatus) || this.$t('node.state.DEFAULT')
      },
      getServiceStatus(state) {
        if ((state & ServiceStatusEnum.SERVICE_STATUS_CREATE_ID) > 0) {
          return this.$t('node.serviceStatus.SERVICE_STATUS_CREATE_ID')
        }
        return undefined
      },
      getStatusColor(stateStr) {
        stateStr = stateStr || 'DEFAULT'
        return this.color[stateStr]
      }
    }
  }
</script>

<style scoped>

</style>

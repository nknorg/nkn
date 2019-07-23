<template>
  <v-card>
    <v-card-title class="headline">{{$t('settings.TITLE')}}</v-card-title>
    <div class="divider"></div>
    <v-card-text>
      <v-layout wrap>
        <v-flex xs12>
          <v-subheader class="pa-0">{{$t('settings.TITLE')}}</v-subheader>
          <NodeRunStatus :node-status="this.nodeStatus"></NodeRunStatus>
          <!--<v-btn color="primary" :loading="restarting"
                 :disabled="restarting" @click="restarting = !restarting">{{$t('RESTART')}}
          </v-btn>-->
        </v-flex>
        <v-flex xs12 md6>
          <v-subheader class="pa-0">{{$t('settings.BENEFICIARY_TITLE')}}</v-subheader>
          <v-text-field :label="$t('BENEFICIARY')" :value="nodeStatus.beneficiaryAddr" box readonly persistent-hint
                        :hint="$t('settings.BENEFICIARY_HINT')">
            <template v-slot:append>
              <v-btn icon small v-clipboard:copy="nodeStatus.beneficiaryAddr">
                <v-icon color="primary" class="far fa-copy" small></v-icon>
              </v-btn>
              <v-btn icon small @click="secretSeedDialog = true">
                <v-icon color="primary" class="far fa-edit" small></v-icon>
              </v-btn>
            </template>
          </v-text-field>
        </v-flex>

      </v-layout>
    </v-card-text>
    <div class="divider"></div>
    <v-card-actions class="pa-3">

    </v-card-actions>
    <BeneficiaryAddrDialog v-model="secretSeedDialog" :on-success="onDialogSuccess"></BeneficiaryAddrDialog>
  </v-card>
</template>

<script>
  import BeneficiaryAddrDialog from '~/components/dialog/BeneficiaryAddr.vue'
  import NodeRunStatus from '~/components/status/NodeRunStatus.vue'
  import MaterialNotification from '~/components/material/Notification'
  import {mapState} from 'vuex'

  export default {
    name: "settings",
    components: {
      NodeRunStatus,
      BeneficiaryAddrDialog,
      MaterialNotification
    },
    computed: mapState({
      nodeStatus: state => state.node.nodeStatus,
    }),
    data() {
      return {
        secretSeedDialog: false,
        restarting: false,
        wallets: null,
        beneficiaryAddr: ''
      }
    },
    methods: {
      onDialogSuccess(data) {
        this.beneficiaryAddr = data.beneficiaryAddr
      }
    }
  }
</script>

<style scoped>

</style>
